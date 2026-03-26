'use client'

import { useEffect, useState, type Dispatch, type ReactNode, type SetStateAction } from 'react'

import { ChevronDown, Loader2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import type { AdminPlugin, AdminPluginHookCatalogGroup, AdminPluginSecretMeta } from '@/lib/api'
import type { Translations } from '@/lib/i18n'

import { PluginAdvancedJsonPanel } from './plugin-advanced-json-panel'
import {
  PluginFrontendAccessEditor,
  type PluginFrontendPermissionCatalogGroup,
  type PluginFrontendSlotCatalogGroup,
} from './plugin-frontend-access-editor'
import {
  PluginCapabilityPolicyEditor,
  type PluginCapabilityPermissionOption,
} from './plugin-capability-policy-editor'
import { PluginHookAccessEditor } from './plugin-hook-access-editor'
import { PluginJSONObjectEditor } from './plugin-json-object-editor'
import { PluginJSONSchemaEditor } from './plugin-json-schema-editor'
import { PluginSecretSchemaEditor } from './plugin-secret-schema-editor'
import type {
  PluginCapabilityPolicyState,
  PluginForm,
  PluginFrontendAccessState,
  PluginHookAccessState,
  PluginJSONSchema,
} from './types'

type PluginEditorDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  editingPlugin: AdminPlugin | null
  pluginForm: PluginForm
  setPluginForm: Dispatch<SetStateAction<PluginForm>>
  runtimeOptions: readonly string[]
  runtimeLabel: (runtime: string, t: Translations) => string
  runtimeAddressLabel: (runtime: string, t: Translations) => string
  runtimeAddressPlaceholder: (runtime: string, t: Translations) => string
  runtimeAddressHint: (runtime: string, mode: 'editor' | 'upload', t: Translations) => string
  configSchema: PluginJSONSchema | null
  secretSchema: PluginJSONSchema | null
  secretMeta: Record<string, AdminPluginSecretMeta | undefined>
  secretDrafts: Record<string, string>
  secretDeleteKeys: string[]
  onSecretDraftChange: (key: string, value: string) => void
  onSecretDeleteToggle: (key: string, checked: boolean) => void
  runtimeParamsSchema: PluginJSONSchema | null
  hookCatalog: AdminPluginHookCatalogGroup[]
  hookAccessState: PluginHookAccessState
  onHookAccessChange: (state: PluginHookAccessState) => void
  resolveHookGroupLabel: (groupKey: string) => string
  capabilityPermissionOptions: PluginCapabilityPermissionOption[]
  capabilityPolicyState: PluginCapabilityPolicyState
  onCapabilityPolicyChange: (state: PluginCapabilityPolicyState) => void
  frontendSlotCatalog: PluginFrontendSlotCatalogGroup[]
  frontendPermissionCatalog: PluginFrontendPermissionCatalogGroup[]
  frontendAccessState: PluginFrontendAccessState
  onFrontendAccessChange: (state: PluginFrontendAccessState) => void
  resolveFrontendSlotGroupLabel: (groupKey: string) => string
  frontendValidationMessage?: string | null
  capabilitiesValid: boolean
  configValid: boolean
  runtimeParamsValid: boolean
  onCapabilitiesChange: (value: string) => void
  onConfigBlur: () => void
  onRuntimeParamsBlur: () => void
  onCapabilitiesBlur: () => void
  submitPlugin: () => void
  isSaving: boolean
  t: Translations
}

type EditorChecklistItem = {
  key: string
  label: string
  ready: boolean
  detail: string
  sectionId: string
}

type EditorSectionState = Record<string, boolean>

const DEFAULT_EDITOR_SECTION_STATE: EditorSectionState = {
  'plugin-editor-basics': true,
  'plugin-editor-config': false,
  'plugin-editor-runtime-params': false,
  'plugin-editor-capabilities': false,
  'plugin-editor-frontend': false,
  'plugin-editor-activation': false,
}

function PluginEditorSection({
  id,
  title,
  description,
  open,
  onToggle,
  children,
}: {
  id: string
  title: string
  description?: string
  open: boolean
  onToggle: () => void
  children: ReactNode
}) {
  return (
    <section id={id} className="scroll-mt-28 overflow-hidden rounded-lg border border-input/70">
      <button
        type="button"
        className="flex w-full items-start justify-between gap-3 bg-muted/20 px-4 py-3 text-left"
        onClick={onToggle}
      >
        <div className="min-w-0 space-y-1">
          <h3 className="text-sm font-semibold">{title}</h3>
          {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ChevronDown
            className={`h-4 w-4 text-muted-foreground transition-transform ${
              open ? 'rotate-180' : ''
            }`}
          />
        </div>
      </button>
      {open ? <div className="space-y-4 border-t border-input/60 p-4">{children}</div> : null}
    </section>
  )
}

export function PluginEditorDialog({
  open,
  onOpenChange,
  editingPlugin,
  pluginForm,
  setPluginForm,
  runtimeOptions,
  runtimeLabel,
  runtimeAddressLabel,
  runtimeAddressPlaceholder,
  runtimeAddressHint,
  configSchema,
  secretSchema,
  secretMeta,
  secretDrafts,
  secretDeleteKeys,
  onSecretDraftChange,
  onSecretDeleteToggle,
  runtimeParamsSchema,
  hookCatalog,
  hookAccessState,
  onHookAccessChange,
  resolveHookGroupLabel,
  capabilityPermissionOptions,
  capabilityPolicyState,
  onCapabilityPolicyChange,
  frontendSlotCatalog,
  frontendPermissionCatalog,
  frontendAccessState,
  onFrontendAccessChange,
  resolveFrontendSlotGroupLabel,
  frontendValidationMessage,
  capabilitiesValid,
  configValid,
  runtimeParamsValid,
  onCapabilitiesChange,
  onConfigBlur,
  onRuntimeParamsBlur,
  onCapabilitiesBlur,
  submitPlugin,
  isSaving,
  t,
}: PluginEditorDialogProps) {
  const [openSections, setOpenSections] = useState<EditorSectionState>(DEFAULT_EDITOR_SECTION_STATE)
  const trimmedName = pluginForm.name.trim()
  const trimmedType = pluginForm.type.trim()
  const trimmedRuntime = pluginForm.runtime.trim()
  const trimmedAddress = pluginForm.address.trim()
  const trimmedPackagePath = pluginForm.package_path.trim()
  const originalRuntime = String(editingPlugin?.runtime || 'grpc').trim()
  const originalAddress = String(editingPlugin?.address || '').trim()
  const originalPackagePath = String(editingPlugin?.package_path || '').trim()
  const requirePackagePath =
    trimmedRuntime === 'js_worker' &&
    (!editingPlugin ||
      trimmedRuntime !== originalRuntime ||
      trimmedAddress !== originalAddress ||
      trimmedPackagePath !== originalPackagePath)
  const requestedPermissionCount = capabilityPolicyState.requestedPermissions.length
  const grantedPermissionSet = new Set(capabilityPolicyState.grantedPermissions)
  const missingPermissionCount = capabilityPolicyState.requestedPermissions.filter(
    (item) => !grantedPermissionSet.has(item)
  ).length
  const editorChecklist: EditorChecklistItem[] = [
    {
      key: 'name',
      label: t.admin.pluginName,
      ready: !!trimmedName,
      detail: trimmedName || t.admin.pluginRequiredName,
      sectionId: 'plugin-editor-basics',
    },
    {
      key: 'type',
      label: t.admin.pluginType,
      ready: !!trimmedType,
      detail: trimmedType || t.admin.pluginRequiredType,
      sectionId: 'plugin-editor-basics',
    },
    {
      key: 'runtime',
      label: t.admin.pluginRuntime,
      ready: !!trimmedRuntime,
      detail: trimmedRuntime ? runtimeLabel(trimmedRuntime, t) : t.admin.pluginRequiredRuntime,
      sectionId: 'plugin-editor-basics',
    },
    {
      key: 'address',
      label: runtimeAddressLabel(pluginForm.runtime, t),
      ready: !!trimmedAddress,
      detail:
        trimmedAddress ||
        (trimmedRuntime === 'js_worker'
          ? t.admin.pluginRequiredEntryScript
          : t.admin.pluginRequiredAddress),
      sectionId: 'plugin-editor-basics',
    },
    ...(trimmedRuntime === 'js_worker'
      ? [
          {
            key: 'package_path',
            label: t.admin.pluginPackagePath,
            ready: !requirePackagePath || !!trimmedPackagePath,
            detail:
              trimmedPackagePath ||
              (requirePackagePath ? t.admin.pluginRequiredPackagePath : t.common.noData),
            sectionId: 'plugin-editor-basics',
          },
        ]
      : []),
    {
      key: 'config',
      label: t.admin.pluginConfig,
      ready: configValid,
      detail: configValid ? t.common.success : t.admin.pluginJsonObjectEditorInvalidRaw,
      sectionId: 'plugin-editor-config',
    },
    {
      key: 'runtime_params',
      label: t.admin.pluginRuntimeParams,
      ready: runtimeParamsValid,
      detail: runtimeParamsValid ? t.common.success : t.admin.pluginJsonObjectEditorInvalidRaw,
      sectionId: 'plugin-editor-runtime-params',
    },
    {
      key: 'capabilities',
      label: t.admin.pluginCapabilities,
      ready: capabilitiesValid,
      detail: capabilitiesValid ? t.common.success : t.admin.pluginHookAccessInvalidJson,
      sectionId: 'plugin-editor-capabilities',
    },
    {
      key: 'frontend',
      label: t.admin.pluginSummaryFrontend,
      ready: !frontendValidationMessage,
      detail: frontendValidationMessage || t.common.success,
      sectionId: 'plugin-editor-frontend',
    },
  ]
  const blockingItems = editorChecklist.filter((item) => !item.ready)
  const canSubmit = blockingItems.length === 0
  const firstBlockingItem = blockingItems[0] || null
  const editorSections = [
    { id: 'plugin-editor-basics', label: t.admin.pluginEditorSectionBasics },
    { id: 'plugin-editor-config', label: t.admin.pluginConfig },
    { id: 'plugin-editor-runtime-params', label: t.admin.pluginRuntimeParams },
    { id: 'plugin-editor-capabilities', label: t.admin.pluginCapabilities },
    { id: 'plugin-editor-frontend', label: t.admin.pluginSummaryFrontend },
    { id: 'plugin-editor-activation', label: t.admin.pluginEditorSectionActivation },
  ]
  const blockedSectionCounts = blockingItems.reduce<Record<string, number>>((acc, item) => {
    acc[item.sectionId] = (acc[item.sectionId] || 0) + 1
    return acc
  }, {})
  const editorSummary = [
    editingPlugin ? t.common.edit : t.common.create,
    trimmedRuntime ? runtimeLabel(trimmedRuntime, t) : null,
    t.admin.pluginSummaryPermissionRequested.replace('{count}', String(requestedPermissionCount)),
    missingPermissionCount > 0
      ? t.admin.pluginSummaryPermissionMissing.replace('{count}', String(missingPermissionCount))
      : null,
  ].filter(Boolean)
  const scrollToSection = (id: string) => {
    setOpenSections((current) => ({ ...current, [id]: true }))
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
  useEffect(() => {
    if (!open) return
    const next = { ...DEFAULT_EDITOR_SECTION_STATE }
    if (firstBlockingItem?.sectionId) {
      next[firstBlockingItem.sectionId] = true
    }
    setOpenSections(next)
  }, [firstBlockingItem?.sectionId, open])

  const setAllSectionsOpen = (expanded: boolean) => {
    setOpenSections(
      Object.keys(DEFAULT_EDITOR_SECTION_STATE).reduce<EditorSectionState>((acc, key) => {
        acc[key] = expanded
        return acc
      }, {})
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto [&_.grid>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>{editingPlugin ? t.admin.pluginEdit : t.admin.pluginAdd}</DialogTitle>
          <DialogDescription>{t.admin.pluginSubtitle}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="rounded-lg border border-input/60 bg-muted/10 p-3">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 space-y-1">
                <p className="text-sm font-medium">
                  {canSubmit
                    ? t.admin.pluginEditorReadinessReady
                    : t.admin.pluginEditorReadinessBlocked}
                </p>
                {editorSummary.length > 0 ? (
                  <p className="text-xs text-muted-foreground">{editorSummary.join(' · ')}</p>
                ) : null}
                <p
                  className={`text-xs ${
                    firstBlockingItem
                      ? 'text-amber-700 dark:text-amber-300'
                      : 'text-muted-foreground'
                  }`}
                >
                  {firstBlockingItem
                    ? `${firstBlockingItem.label} · ${firstBlockingItem.detail}`
                    : t.admin.pluginEditorSaveReadyHint}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setAllSectionsOpen(true)}
                >
                  {t.common.expand}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setAllSectionsOpen(false)}
                >
                  {t.common.collapse}
                </Button>
                {editorSections.map((section) => (
                  <Button
                    key={section.id}
                    type="button"
                    variant="outline"
                    size="sm"
                    className={
                      (blockedSectionCounts[section.id] || 0) > 0
                        ? 'border-input bg-secondary/60 hover:bg-secondary/80'
                        : undefined
                    }
                    onClick={() => scrollToSection(section.id)}
                  >
                    <span>
                      {section.label}
                      {(blockedSectionCounts[section.id] || 0) > 0
                        ? ` (${blockedSectionCounts[section.id]})`
                        : ''}
                    </span>
                  </Button>
                ))}
              </div>
            </div>
          </div>

          <PluginEditorSection
            id="plugin-editor-basics"
            title={t.admin.pluginEditorSectionBasics}
            description={t.admin.pluginSubtitle}
            open={!!openSections['plugin-editor-basics']}
            onToggle={() =>
              setOpenSections((current) => ({
                ...current,
                'plugin-editor-basics': !current['plugin-editor-basics'],
              }))
            }
          >
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label>{t.admin.pluginName}</Label>
                <Input
                  value={pluginForm.name}
                  onChange={(e) => setPluginForm((p) => ({ ...p, name: e.target.value }))}
                  disabled={!!editingPlugin}
                />
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pluginDisplayName}</Label>
                <Input
                  value={pluginForm.display_name}
                  onChange={(e) => setPluginForm((p) => ({ ...p, display_name: e.target.value }))}
                />
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pluginType}</Label>
                <Input
                  value={pluginForm.type}
                  onChange={(e) => setPluginForm((p) => ({ ...p, type: e.target.value }))}
                />
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pluginRuntime}</Label>
                <Select
                  value={pluginForm.runtime}
                  onValueChange={(value) => setPluginForm((p) => ({ ...p, runtime: value }))}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {runtimeOptions.map((item) => (
                      <SelectItem key={item} value={item}>
                        {runtimeLabel(item, t)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pluginVersionLabel}</Label>
                <Input
                  value={pluginForm.version}
                  onChange={(e) => setPluginForm((p) => ({ ...p, version: e.target.value }))}
                />
              </div>
            </div>
            {pluginForm.runtime === 'js_worker' ? (
              <div className="space-y-2">
                <Label>{t.admin.pluginPackagePath}</Label>
                <Input
                  value={pluginForm.package_path}
                  placeholder={t.admin.pluginPackagePathPlaceholder}
                  onChange={(e) => setPluginForm((p) => ({ ...p, package_path: e.target.value }))}
                />
                <p className="text-xs text-muted-foreground">
                  {t.admin.pluginPackagePathHintEditor}
                </p>
              </div>
            ) : null}
            <div className="space-y-2">
              <Label>{runtimeAddressLabel(pluginForm.runtime, t)}</Label>
              <Input
                value={pluginForm.address}
                placeholder={runtimeAddressPlaceholder(pluginForm.runtime, t)}
                onChange={(e) => setPluginForm((p) => ({ ...p, address: e.target.value }))}
              />
              <p className="text-xs text-muted-foreground">
                {runtimeAddressHint(pluginForm.runtime, 'editor', t)}
              </p>
            </div>
            <div className="space-y-2">
              <Label>{t.admin.pluginDescription}</Label>
              <Textarea
                value={pluginForm.description}
                onChange={(e) => setPluginForm((p) => ({ ...p, description: e.target.value }))}
                rows={3}
              />
            </div>
          </PluginEditorSection>
          <PluginEditorSection
            id="plugin-editor-config"
            title={t.admin.pluginConfig}
            description={t.admin.pluginConfigVisualEditorDesc}
            open={!!openSections['plugin-editor-config']}
            onToggle={() =>
              setOpenSections((current) => ({
                ...current,
                'plugin-editor-config': !current['plugin-editor-config'],
              }))
            }
          >
            {configSchema ? (
              <PluginJSONSchemaEditor
                title={configSchema.title || t.admin.pluginConfigPresetEditor}
                description={configSchema.description || t.admin.pluginConfigPresetEditorDesc}
                schema={configSchema}
                value={pluginForm.config}
                onChange={(value) => setPluginForm((p) => ({ ...p, config: value }))}
                disabled={!configValid}
                disabledReason={!configValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined}
                t={t}
              />
            ) : null}
            {secretSchema ? (
              <PluginSecretSchemaEditor
                title={secretSchema.title || t.admin.pluginSecretsPresetEditor}
                description={secretSchema.description || t.admin.pluginSecretsPresetEditorDesc}
                schema={secretSchema}
                secretMeta={secretMeta}
                drafts={secretDrafts}
                deleteKeys={secretDeleteKeys}
                onDraftChange={onSecretDraftChange}
                onDeleteToggle={onSecretDeleteToggle}
                t={t}
              />
            ) : null}
            <PluginJSONObjectEditor
              title={
                configSchema ? t.admin.pluginConfigVisualEditorExtra : t.admin.pluginConfigVisualEditor
              }
              description={
                configSchema
                  ? t.admin.pluginConfigVisualEditorExtraDesc
                  : t.admin.pluginConfigVisualEditorDesc
              }
              value={pluginForm.config}
              onChange={(value) => setPluginForm((p) => ({ ...p, config: value }))}
              excludedKeys={configSchema?.fields.map((field) => field.key) || []}
              emptyMessage={t.admin.pluginJsonObjectEditorNoExtraFields}
              disabled={!configValid}
              disabledReason={!configValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined}
              t={t}
            />
            <PluginAdvancedJsonPanel
              title={`${t.admin.pluginAdvancedJsonEditor} · ${t.admin.pluginConfig}`}
              fieldLabel={t.admin.pluginConfig}
              description={t.admin.pluginAdvancedJsonEditorDesc}
              value={pluginForm.config}
              onChange={(value) => setPluginForm((p) => ({ ...p, config: value }))}
              onBlur={onConfigBlur}
              rows={8}
              invalid={!configValid}
              invalidMessage={!configValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined}
              hints={[t.admin.pluginJsonAutoFormatHint]}
              t={t}
            />
          </PluginEditorSection>

          <PluginEditorSection
            id="plugin-editor-runtime-params"
            title={t.admin.pluginRuntimeParams}
            description={t.admin.pluginRuntimeParamsVisualEditorDesc}
            open={!!openSections['plugin-editor-runtime-params']}
            onToggle={() =>
              setOpenSections((current) => ({
                ...current,
                'plugin-editor-runtime-params': !current['plugin-editor-runtime-params'],
              }))
            }
          >
            {runtimeParamsSchema ? (
              <PluginJSONSchemaEditor
                title={runtimeParamsSchema.title || t.admin.pluginRuntimeParamsPresetEditor}
                description={
                  runtimeParamsSchema.description || t.admin.pluginRuntimeParamsPresetEditorDesc
                }
                schema={runtimeParamsSchema}
                value={pluginForm.runtime_params}
                onChange={(value) => setPluginForm((p) => ({ ...p, runtime_params: value }))}
                disabled={!runtimeParamsValid}
                disabledReason={
                  !runtimeParamsValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined
                }
                t={t}
              />
            ) : null}
            <PluginJSONObjectEditor
              title={
                runtimeParamsSchema
                  ? t.admin.pluginRuntimeParamsVisualEditorExtra
                  : t.admin.pluginRuntimeParamsVisualEditor
              }
              description={
                runtimeParamsSchema
                  ? t.admin.pluginRuntimeParamsVisualEditorExtraDesc
                  : t.admin.pluginRuntimeParamsVisualEditorDesc
              }
              value={pluginForm.runtime_params}
              onChange={(value) => setPluginForm((p) => ({ ...p, runtime_params: value }))}
              excludedKeys={runtimeParamsSchema?.fields.map((field) => field.key) || []}
              emptyMessage={t.admin.pluginJsonObjectEditorNoExtraFields}
              disabled={!runtimeParamsValid}
              disabledReason={
                !runtimeParamsValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined
              }
              t={t}
            />
            <PluginAdvancedJsonPanel
              title={`${t.admin.pluginAdvancedJsonEditor} · ${t.admin.pluginRuntimeParams}`}
              fieldLabel={t.admin.pluginRuntimeParams}
              description={t.admin.pluginAdvancedJsonEditorDesc}
              value={pluginForm.runtime_params}
              onChange={(value) =>
                setPluginForm((p) => ({ ...p, runtime_params: value }))
              }
              onBlur={onRuntimeParamsBlur}
              rows={6}
              invalid={!runtimeParamsValid}
              invalidMessage={
                !runtimeParamsValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined
              }
              hints={[t.admin.pluginRuntimeParamsVisualEditorHint, t.admin.pluginJsonAutoFormatHint]}
              t={t}
            />
          </PluginEditorSection>
          <PluginEditorSection
            id="plugin-editor-capabilities"
            title={t.admin.pluginCapabilities}
            description={t.admin.pluginCapabilitiesHint}
            open={!!openSections['plugin-editor-capabilities']}
            onToggle={() =>
              setOpenSections((current) => ({
                ...current,
                'plugin-editor-capabilities': !current['plugin-editor-capabilities'],
              }))
            }
          >
            <PluginHookAccessEditor
              hookCatalog={hookCatalog}
              hookAccessState={hookAccessState}
              onHookAccessChange={onHookAccessChange}
              resolveHookGroupLabel={resolveHookGroupLabel}
              disabled={!capabilitiesValid}
              disabledReason={!capabilitiesValid ? t.admin.pluginHookAccessInvalidJson : undefined}
              t={t}
            />
            <PluginCapabilityPolicyEditor
              capabilityPolicyState={capabilityPolicyState}
              onCapabilityPolicyChange={onCapabilityPolicyChange}
              permissionOptions={capabilityPermissionOptions}
              showPermissionEditor
              disabled={!capabilitiesValid}
              disabledReason={!capabilitiesValid ? t.admin.pluginHookAccessInvalidJson : undefined}
              t={t}
            />
            <PluginAdvancedJsonPanel
              title={`${t.admin.pluginAdvancedJsonEditor} · ${t.admin.pluginCapabilities}`}
              fieldLabel={t.admin.pluginCapabilities}
              description={t.admin.pluginAdvancedJsonEditorDesc}
              value={pluginForm.capabilities}
              onChange={onCapabilitiesChange}
              onBlur={onCapabilitiesBlur}
              rows={10}
              invalid={!capabilitiesValid}
              invalidMessage={!capabilitiesValid ? t.admin.pluginHookAccessInvalidJson : undefined}
              hints={[t.admin.pluginCapabilitiesHint]}
              t={t}
            />
          </PluginEditorSection>

          <PluginEditorSection
            id="plugin-editor-frontend"
            title={t.admin.pluginSummaryFrontend}
            description={t.admin.pluginFrontendAccessDesc}
            open={!!openSections['plugin-editor-frontend']}
            onToggle={() =>
              setOpenSections((current) => ({
                ...current,
                'plugin-editor-frontend': !current['plugin-editor-frontend'],
              }))
            }
          >
            <PluginFrontendAccessEditor
              slotCatalog={frontendSlotCatalog}
              permissionCatalog={frontendPermissionCatalog}
              frontendAccessState={frontendAccessState}
              onFrontendAccessChange={onFrontendAccessChange}
              resolveSlotGroupLabel={resolveFrontendSlotGroupLabel}
              disabled={!capabilitiesValid}
              disabledReason={
                !capabilitiesValid ? t.admin.pluginFrontendAccessInvalidJson : undefined
              }
              validationMessage={frontendValidationMessage}
              t={t}
            />
          </PluginEditorSection>

          <PluginEditorSection
            id="plugin-editor-activation"
            title={t.admin.pluginEditorSectionActivation}
            open={!!openSections['plugin-editor-activation']}
            onToggle={() =>
              setOpenSections((current) => ({
                ...current,
                'plugin-editor-activation': !current['plugin-editor-activation'],
              }))
            }
          >
            <div className="space-y-4">
              <div className="flex items-center justify-between gap-3 rounded-md border border-input/60 bg-muted/10 p-3">
                <div className="space-y-1">
                  <Label>{t.admin.pluginEnabled}</Label>
                  <p className="text-xs text-muted-foreground">
                    {editingPlugin ? t.common.edit : t.common.create} ·{' '}
                    {pluginForm.enabled ? t.admin.enabled : t.admin.disabled}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {pluginForm.enabled
                      ? t.admin.pluginEditorSaveReadyHint
                      : t.admin.pluginEditorSaveBlocked}
                  </p>
                </div>
                <Switch
                  checked={pluginForm.enabled}
                  onCheckedChange={(checked) => setPluginForm((p) => ({ ...p, enabled: checked }))}
                />
              </div>
            </div>
          </PluginEditorSection>
        </div>
        <DialogFooter className="border-t border-input/60 pt-4">
          <div className="mr-auto flex flex-wrap items-center gap-2">
            <p className="text-xs text-muted-foreground">
              {canSubmit
                ? t.admin.pluginEditorSaveReadyHint
                : firstBlockingItem?.detail || t.admin.pluginEditorSaveBlocked}
            </p>
            {!canSubmit && firstBlockingItem ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => scrollToSection(firstBlockingItem.sectionId)}
              >
                {firstBlockingItem.label}
              </Button>
            ) : null}
          </div>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t.common.cancel}
          </Button>
          <Button onClick={submitPlugin} disabled={isSaving || !canSubmit}>
            {isSaving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            {editingPlugin ? t.common.save : t.common.create}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
