'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import type { Dispatch, ReactNode, SetStateAction } from 'react'

import { ChevronDown, FileUp, Loader2, X } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import type { AdminPlugin, AdminPluginHookCatalogGroup, PluginPermissionRequest } from '@/lib/api'
import type { Translations } from '@/lib/i18n'
import { resolveManifestLocalizedString } from '@/lib/package-manifest-schema'

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
import { PluginGovernanceNotice } from './plugin-governance-summary'
import type { PluginUploadConflictSummary } from './plugin-upload-conflicts'
import type {
  MarketPluginInstallContext,
  PluginCapabilityPolicyState,
  PluginFrontendAccessState,
  PluginHookAccessState,
  PluginJSONSchema,
  UploadForm,
  UploadPermissionPreview,
} from './types'

type PluginUploadDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  uploadInputKey: number
  uploadFile: File | null
  onUploadFilePicked: (file: File | null) => void
  previewPending: boolean
  uploadPending: boolean
  uploadPreview: UploadPermissionPreview | null
  uploadConflictSummary: PluginUploadConflictSummary
  uploadGrantedPermissions: string[]
  setUploadGrantedPermissions: Dispatch<SetStateAction<string[]>>
  normalizeStringList: (values: string[]) => string[]
  resolvePluginPermissionTitle: (permission: PluginPermissionRequest, t: Translations) => string
  resolvePluginPermissionDescription: (
    permission: PluginPermissionRequest,
    t: Translations
  ) => string
  uploadForm: UploadForm
  setUploadForm: Dispatch<SetStateAction<UploadForm>>
  plugins: AdminPlugin[]
  runtimeOptions: readonly string[]
  runtimeLabel: (runtime: string, t: Translations) => string
  runtimeAddressLabel: (runtime: string, t: Translations) => string
  runtimeAddressPlaceholder: (runtime: string, t: Translations) => string
  runtimeAddressHint: (runtime: string, mode: 'editor' | 'upload', t: Translations) => string
  configSchema: PluginJSONSchema | null
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
  submitUpload: () => void
  marketInstallContext?: MarketPluginInstallContext | null
  locale: string
  t: Translations
}

type UploadStepState = {
  package: boolean
  target: boolean
  config: boolean
  release: boolean
}

type UploadStepStatus = 'complete' | 'attention' | 'pending'

type UploadChecklistItem = {
  key: string
  label: string
  ready: boolean
  detail: string
  stepKey: keyof UploadStepState
  sectionId: string
}

type PermissionPreviewFilter = 'all' | 'required' | 'optional' | 'granted' | 'pending'

const DEFAULT_UPLOAD_STEP_STATE: UploadStepState = {
  package: true,
  target: true,
  config: false,
  release: false,
}

function UploadStepSection({
  id,
  step,
  title,
  description,
  status,
  statusLabel,
  open,
  onToggle,
  children,
}: {
  id?: string
  step: string
  title: string
  description?: string
  status: UploadStepStatus
  statusLabel: string
  open: boolean
  onToggle: () => void
  children: ReactNode
}) {
  return (
    <section id={id} className="scroll-mt-28 overflow-hidden rounded-lg border border-input">
      <button
        type="button"
        className="flex w-full items-start justify-between gap-3 bg-muted/20 px-3 py-2 text-left"
        onClick={onToggle}
      >
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full border border-input px-1.5 text-[11px] font-medium">
              {step}
            </span>
            <p className="text-sm font-medium">{title}</p>
            <span className="text-xs text-muted-foreground">{statusLabel}</span>
          </div>
          {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
        </div>
        <ChevronDown
          className={`mt-0.5 h-4 w-4 shrink-0 text-muted-foreground transition-transform ${
            open ? 'rotate-180' : ''
          }`}
        />
      </button>
      {open ? <div className="space-y-4 border-t border-input/60 px-3 py-3">{children}</div> : null}
    </section>
  )
}

function formatMarketDisplayValue(value: unknown): string {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return trimmed || '-'
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return '-'
}

function formatUploadFileSize(bytes: number | undefined): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function joinSummaryItems(items: Array<string | null | undefined | false>): string {
  return items
    .filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    .join(' · ')
}

function marketCompatibilityVariant(
  compatible: boolean | undefined,
  legacyDefaultsApplied: boolean | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  if (compatible === false) return 'destructive'
  if (legacyDefaultsApplied) return 'secondary'
  if (compatible === true) return 'active'
  return 'outline'
}

function marketCompatibilityLabel(
  compatible: boolean | undefined,
  legacyDefaultsApplied: boolean | undefined,
  t: Translations
): string {
  if (compatible === false) return t.admin.pluginDiagnosticsCompatibilityStateIncompatible
  if (legacyDefaultsApplied) return t.admin.pluginDiagnosticsCompatibilityStateLegacy
  if (compatible === true) return t.admin.pluginDiagnosticsCompatibilityStateCompatible
  return t.common.noData
}

function UploadSummaryCard({
  title,
  description,
  badge,
  children,
}: {
  title: string
  description?: string
  badge?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="rounded-md border border-input/60 bg-muted/10 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0 space-y-1">
          <p className="text-sm font-medium">{title}</p>
          {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
        </div>
        {badge ? <div className="shrink-0">{badge}</div> : null}
      </div>
      <div className="mt-3 min-w-0">{children}</div>
    </div>
  )
}

function UploadActionCard({
  title,
  description,
  badge,
  actionLabel,
  onAction,
  tone = 'default',
  children,
}: {
  title: string
  description?: string
  badge?: ReactNode
  actionLabel?: string
  onAction?: () => void
  tone?: 'default' | 'danger'
  children?: ReactNode
}) {
  return (
    <div
      className={`rounded-md border p-3 ${
        tone === 'danger' ? 'border-destructive/30 bg-destructive/5' : 'border-input/60 bg-background'
      }`}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0 space-y-1">
          <p className="text-sm font-medium">{title}</p>
          {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
        </div>
        {badge ? <div className="shrink-0">{badge}</div> : null}
      </div>
      {children ? <div className="mt-3 min-w-0">{children}</div> : null}
      {actionLabel && onAction ? (
        <div className="mt-3">
          <Button type="button" variant="outline" size="sm" onClick={onAction}>
            {actionLabel}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function UploadSummaryValue({
  label,
  value,
  mono = false,
}: {
  label: string
  value: ReactNode
  mono?: boolean
}) {
  return (
    <div className="min-w-0 space-y-1">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={mono ? 'break-all font-mono text-sm font-medium' : 'break-all text-sm font-medium'}>
        {value}
      </dd>
    </div>
  )
}

export function PluginUploadDialog({
  open,
  onOpenChange,
  uploadInputKey,
  uploadFile,
  onUploadFilePicked,
  previewPending,
  uploadPending,
  uploadPreview,
  uploadConflictSummary,
  uploadGrantedPermissions,
  setUploadGrantedPermissions,
  normalizeStringList,
  resolvePluginPermissionTitle,
  resolvePluginPermissionDescription,
  uploadForm,
  setUploadForm,
  plugins,
  runtimeOptions,
  runtimeLabel,
  runtimeAddressLabel,
  runtimeAddressPlaceholder,
  runtimeAddressHint,
  configSchema,
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
  submitUpload,
  marketInstallContext,
  locale,
  t,
}: PluginUploadDialogProps) {
  const uploadFileInputRef = useRef<HTMLInputElement | null>(null)
  const [openSteps, setOpenSteps] = useState<UploadStepState>(DEFAULT_UPLOAD_STEP_STATE)
  const [permissionFilter, setPermissionFilter] = useState<PermissionPreviewFilter>('all')
  const [permissionSearchText, setPermissionSearchText] = useState('')
  const adminPageConflicts = uploadConflictSummary.pageConflicts.filter(
    (item) => item.area === 'admin'
  )
  const userPageConflicts = uploadConflictSummary.pageConflicts.filter(
    (item) => item.area === 'user'
  )
  const isMarketMode = !!marketInstallContext
  const marketRelease =
    marketInstallContext?.release && typeof marketInstallContext.release === 'object'
      ? marketInstallContext.release
      : null
  const marketCompatibility =
    marketInstallContext?.compatibility && typeof marketInstallContext.compatibility === 'object'
      ? marketInstallContext.compatibility
      : null
  const marketTargetState =
    marketInstallContext?.target_state && typeof marketInstallContext.target_state === 'object'
      ? marketInstallContext.target_state
      : null
  const marketWarnings = Array.isArray(marketInstallContext?.warnings)
    ? marketInstallContext?.warnings.filter(
        (item): item is string => typeof item === 'string' && item.trim() !== ''
      )
    : []
  const marketReleaseNotes = resolveManifestLocalizedString((marketRelease as any)?.release_notes, locale)
  const marketCompatible =
    typeof marketCompatibility?.compatible === 'boolean' ? marketCompatibility.compatible : undefined
  const marketLegacyDefaultsApplied =
    typeof marketCompatibility?.legacy_defaults_applied === 'boolean'
      ? marketCompatibility.legacy_defaults_applied
      : undefined
  const marketCompatibilityReason = String(marketCompatibility?.reason || '').trim()
  const marketTargetInstalled = !!(marketTargetState as any)?.installed
  const marketTargetUpdateAvailable = !!(marketTargetState as any)?.update_available
  const marketTargetStatusLabel = marketTargetInstalled
    ? marketTargetUpdateAvailable
      ? t.admin.pluginVersionUpdateAvailable
      : t.admin.pluginVersionCurrentActive
    : t.admin.pluginUploadNewPlugin
  const marketTargetStatusVariant: 'default' | 'secondary' | 'destructive' | 'outline' | 'active' =
    marketTargetInstalled
      ? marketTargetUpdateAvailable
        ? 'secondary'
        : 'outline'
      : 'active'
  const selectedTargetPlugin =
    plugins.find((plugin) => String(plugin.id) === String(uploadForm.plugin_id || '')) || null
  const totalPageConflictCount = adminPageConflicts.length + userPageConflicts.length
  const conflictDetailsAvailable =
    !!uploadConflictSummary.nameConflict ||
    !!uploadConflictSummary.manifestPagePaths.adminPath ||
    !!uploadConflictSummary.manifestPagePaths.userPath
  const permissionPreviewSummary = useMemo(() => {
    const requested = Array.isArray(uploadPreview?.requested_permissions)
      ? uploadPreview.requested_permissions
      : []
    const granted = new Set(normalizeStringList(uploadGrantedPermissions))
    let requiredCount = 0
    let grantedCount = 0
    let defaultGrantedCount = 0
    requested.forEach((permission) => {
      const key = String(permission?.key || '')
        .trim()
        .toLowerCase()
      if (!key) return
      const required = !!permission?.required
      const defaultGranted =
        Array.isArray(uploadPreview?.default_granted_permissions) &&
        uploadPreview.default_granted_permissions.includes(key)
      if (required) {
        requiredCount += 1
      }
      if (!required && defaultGranted) {
        defaultGrantedCount += 1
      }
      if (required || granted.has(key)) {
        grantedCount += 1
      }
    })
    return {
      requestedCount: requested.length,
      requiredCount,
      optionalCount: Math.max(requested.length - requiredCount, 0),
      grantedCount,
      defaultGrantedCount,
    }
  }, [normalizeStringList, uploadGrantedPermissions, uploadPreview])
  const requestedPermissions = useMemo(
    () =>
      Array.isArray(uploadPreview?.requested_permissions) ? uploadPreview.requested_permissions : [],
    [uploadPreview]
  )
  const normalizedGrantedPermissions = useMemo(
    () => normalizeStringList(uploadGrantedPermissions),
    [normalizeStringList, uploadGrantedPermissions]
  )
  const normalizedDefaultGrantedPermissions = useMemo(
    () =>
      normalizeStringList(
        Array.isArray(uploadPreview?.default_granted_permissions)
          ? uploadPreview.default_granted_permissions
          : []
      ),
    [normalizeStringList, uploadPreview]
  )
  const recommendedGrantedPermissions = useMemo(
    () =>
      normalizeStringList([
        ...normalizedDefaultGrantedPermissions,
        ...requestedPermissions
          .filter((item) => !!item?.required)
          .map((item) => String(item?.key || '')),
      ]),
    [normalizeStringList, normalizedDefaultGrantedPermissions, requestedPermissions]
  )
  const permissionPreviewItems = useMemo(
    () =>
      requestedPermissions
        .map((permission) => {
          const key = String(permission?.key || '')
            .trim()
            .toLowerCase()
          if (!key) return null
          const required = !!permission.required
          const defaultGranted = normalizedDefaultGrantedPermissions.includes(key)
          const checked = required || normalizedGrantedPermissions.includes(key)
          return {
            permission,
            key,
            required,
            defaultGranted,
            checked,
            title: resolvePluginPermissionTitle(permission, t),
            description: resolvePluginPermissionDescription(permission, t),
            reason: String(permission?.reason || '').trim(),
          }
        })
        .filter(
          (
            item
          ): item is {
            permission: PluginPermissionRequest
            key: string
            required: boolean
            defaultGranted: boolean
            checked: boolean
            title: string
            description: string
            reason: string
          } => !!item
        ),
    [
      normalizedDefaultGrantedPermissions,
      normalizedGrantedPermissions,
      requestedPermissions,
      resolvePluginPermissionDescription,
      resolvePluginPermissionTitle,
      t,
    ]
  )
  const pendingPermissionCount = useMemo(
    () => permissionPreviewItems.filter((item) => !item.required && !item.checked).length,
    [permissionPreviewItems]
  )
  const filteredPermissionPreviewItems = useMemo(() => {
    const search = permissionSearchText.trim().toLowerCase()
    return permissionPreviewItems.filter((item) => {
      if (permissionFilter === 'required' && !item.required) return false
      if (permissionFilter === 'optional' && item.required) return false
      if (permissionFilter === 'granted' && !item.checked) return false
      if (permissionFilter === 'pending' && (item.required || item.checked)) return false
      if (!search) return true
      const haystack = [item.title, item.key, item.description, item.reason]
        .map((value) => String(value || '').toLowerCase())
        .join('\n')
      return haystack.includes(search)
    })
  }, [permissionFilter, permissionPreviewItems, permissionSearchText])
  const packagePreviewStateLabel = !uploadFile
    ? t.common.noData
    : previewPending
      ? t.common.processing
      : uploadPreview
        ? t.common.success
        : t.common.warning
  const packagePreviewStateVariant: 'default' | 'secondary' | 'destructive' | 'outline' | 'active' =
    !uploadFile ? 'outline' : previewPending ? 'secondary' : uploadPreview ? 'active' : 'secondary'

  useEffect(() => {
    if (open) {
      setPermissionFilter('all')
      setPermissionSearchText('')
      setOpenSteps(
        isMarketMode
          ? {
              package: true,
              target: true,
              config: false,
              release: false,
            }
          : DEFAULT_UPLOAD_STEP_STATE
      )
    }
  }, [open, isMarketMode])

  useEffect(() => {
    if (uploadPreview) {
      setOpenSteps((prev) => ({
        ...prev,
        config: isMarketMode ? prev.config : true,
        release: isMarketMode,
      }))
    }
  }, [isMarketMode, uploadPreview])

  const packageReady = useMemo(() => {
    if (isMarketMode) {
      return !!marketInstallContext
    }
    return !!uploadFile && !previewPending && !!uploadPreview
  }, [isMarketMode, marketInstallContext, previewPending, uploadFile, uploadPreview])

  const targetReady = useMemo(() => {
    if (uploadForm.name.trim() === '' || uploadForm.runtime.trim() === '' || uploadForm.version.trim() === '') {
      return false
    }
    if (!isMarketMode && uploadConflictSummary.hasConflict) {
      return false
    }
    return true
  }, [isMarketMode, uploadConflictSummary.hasConflict, uploadForm.name, uploadForm.runtime, uploadForm.version])

  const configReady = isMarketMode ? true : configValid && runtimeParamsValid && capabilitiesValid
  const releaseReady = isMarketMode ? !!marketRelease : true

  const readinessItems = useMemo<UploadChecklistItem[]>(
    () => [
      {
        key: 'package',
        label: t.admin.pluginUploadStepPackage,
        ready: packageReady,
        detail: packageReady
          ? t.admin.pluginUploadReadinessPackageReady
          : t.admin.pluginUploadReadinessPackagePending,
        stepKey: 'package',
        sectionId: 'plugin-upload-step-package',
      },
      {
        key: 'target',
        label: t.admin.pluginUploadStepTarget,
        ready: targetReady,
        detail: targetReady
          ? t.admin.pluginUploadReadinessTargetReady
          : t.admin.pluginUploadReadinessTargetPending,
        stepKey: 'target',
        sectionId: 'plugin-upload-step-target',
      },
      {
        key: 'config',
        label: isMarketMode ? t.admin.pluginUploadStepRelease : t.admin.pluginUploadStepConfig,
        ready: isMarketMode ? releaseReady : configReady,
        detail:
          isMarketMode
            ? releaseReady
              ? t.admin.pluginUploadReadinessReleaseReady
              : t.admin.pluginUploadReadinessReleasePending
            : configReady
              ? t.admin.pluginUploadReadinessConfigReady
              : t.admin.pluginUploadReadinessConfigPending,
        stepKey: isMarketMode ? 'release' : 'config',
        sectionId: isMarketMode ? 'plugin-upload-step-release' : 'plugin-upload-step-config',
      },
    ],
    [configReady, isMarketMode, packageReady, releaseReady, t.admin, targetReady]
  )
  const blockedChecklistItems = readinessItems.filter((item) => !item.ready)

  const submitReady = blockedChecklistItems.length === 0

  const packageStepStatus: UploadStepStatus = packageReady ? 'complete' : 'attention'
  const targetStepStatus: UploadStepStatus = targetReady ? 'complete' : 'attention'
  const configStepStatus: UploadStepStatus = configReady ? 'complete' : 'attention'
  const releaseStepStatus: UploadStepStatus = releaseReady ? 'complete' : 'pending'

  const resolveStepStatusLabel = (status: UploadStepStatus): string => {
    switch (status) {
      case 'complete':
        return t.admin.pluginUploadStepStatusComplete
      case 'attention':
        return t.admin.pluginUploadStepStatusAttention
      default:
        return t.admin.pluginUploadStepStatusPending
    }
  }
  const resolvePermissionFilterLabel = (value: PermissionPreviewFilter): string => {
    switch (value) {
      case 'required':
        return t.admin.pluginPermissionRequired
      case 'optional':
        return t.admin.pluginPermissionOptional
      case 'granted':
        return t.admin.pluginPermissionFilterGranted
      case 'pending':
        return t.admin.pluginPermissionFilterPending
      default:
        return t.admin.pluginPermissionFilterAll
    }
  }

  const openAndScrollToStep = (stepKey: keyof UploadStepState, id: string) => {
    setOpenSteps((prev) => ({ ...prev, [stepKey]: true }))
    window.requestAnimationFrame(() => {
      document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  }
  const selectTargetPlugin = (pluginId: number) => {
    setUploadForm((prev) => ({ ...prev, plugin_id: String(pluginId) }))
    openAndScrollToStep('target', 'plugin-upload-step-target')
  }
  const reopenPackagePicker = () => {
    setOpenSteps((prev) => ({ ...prev, package: true }))
    uploadFileInputRef.current?.click()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto [&_.grid>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>{isMarketMode ? t.admin.pluginMarketInstall : t.admin.pluginUpload}</DialogTitle>
          <DialogDescription>
            {isMarketMode ? t.admin.pluginMarketInstallDesc : t.admin.pluginUploadFile}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <UploadStepSection
            id="plugin-upload-step-package"
            step="1"
            title={t.admin.pluginUploadStepPackage}
            description={t.admin.pluginUploadStepPackageDesc}
            status={packageStepStatus}
            statusLabel={resolveStepStatusLabel(packageStepStatus)}
            open={openSteps.package}
            onToggle={() => setOpenSteps((prev) => ({ ...prev, package: !prev.package }))}
          >
          {isMarketMode ? (
            <div className="space-y-3 rounded-md border border-input/60 bg-muted/10 p-3">
              <p className="text-xs text-muted-foreground">
                {[
                  formatMarketDisplayValue(marketInstallContext?.source.source_id),
                  formatMarketDisplayValue(uploadForm.runtime),
                  marketWarnings.length > 0
                    ? `${t.admin.pluginMarketInstallWarnings}: ${marketWarnings.length}`
                    : null,
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </p>
              <dl className="grid gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
                <UploadSummaryValue
                  label={t.admin.pluginMarketInstallSource}
                  value={
                    formatMarketDisplayValue(
                      marketInstallContext?.source.name || marketInstallContext?.source.source_id
                    )
                  }
                />
                <UploadSummaryValue
                  label={t.admin.pluginMarketInstallBaseUrl}
                  value={formatMarketDisplayValue(marketInstallContext?.source.base_url)}
                  mono
                />
                <UploadSummaryValue
                  label={t.admin.pluginName}
                  value={formatMarketDisplayValue(marketInstallContext?.coordinates.name)}
                  mono
                />
                <UploadSummaryValue
                  label={t.admin.pluginUploadVersion}
                  value={formatMarketDisplayValue(marketInstallContext?.coordinates.version)}
                />
                <UploadSummaryValue
                  label={t.admin.pluginDisplayName}
                  value={formatMarketDisplayValue(
                    resolveManifestLocalizedString((marketRelease as any)?.title, locale) ||
                      uploadForm.display_name
                  )}
                />
              </dl>
              {marketWarnings.length > 0 ? (
                <details className="rounded-md border border-input/60 bg-background p-3">
                  <summary className="cursor-pointer text-xs font-medium">
                    {t.admin.pluginMarketInstallWarnings}
                  </summary>
                  <div className="mt-2 max-h-32 space-y-1 overflow-auto pr-1">
                    {marketWarnings.map((warning, index) => (
                      <p key={`${warning}-${index}`} className="text-xs text-muted-foreground">
                        {warning}
                      </p>
                    ))}
                  </div>
                </details>
              ) : null}
            </div>
          ) : (
            <>
          <div className="space-y-2">
            <Label>{t.admin.pluginUploadFile}</Label>
            <input
              key={uploadInputKey}
              ref={uploadFileInputRef}
              type="file"
              className="hidden"
              onChange={(e) => onUploadFilePicked(e.target.files?.[0] || null)}
            />
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => uploadFileInputRef.current?.click()}
                disabled={previewPending || uploadPending}
              >
                <FileUp className="mr-2 h-4 w-4" />
                {t.admin.pluginUploadChooseFile}
              </Button>
              {uploadFile ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    if (uploadFileInputRef.current) {
                      uploadFileInputRef.current.value = ''
                    }
                    onUploadFilePicked(null)
                  }}
                  disabled={previewPending || uploadPending}
                >
                  <X className="mr-2 h-4 w-4" />
                  {t.admin.pluginUploadClearFile}
                </Button>
              ) : null}
              <span className="break-all text-sm text-muted-foreground">
                {uploadFile?.name || t.admin.pluginUploadNoFileSelected}
              </span>
            </div>
          </div>
          <UploadSummaryCard
            title={t.admin.pluginPermissionPreview}
            description={t.admin.pluginPermissionPreviewHint}
            badge={
              previewPending ? (
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  {t.common.processing}
                </span>
              ) : (
                <span className="text-xs text-muted-foreground">{packagePreviewStateLabel}</span>
              )
            }
          >
            {!uploadFile ? (
              <PluginGovernanceNotice tone="neutral">
                {t.admin.pluginPermissionSelectFile}
              </PluginGovernanceNotice>
            ) : previewPending ? (
              <PluginGovernanceNotice tone="neutral">{t.common.loading}</PluginGovernanceNotice>
            ) : uploadPreview && uploadPreview.requested_permissions.length > 0 ? (
              <div className="space-y-3">
                {permissionPreviewSummary.defaultGrantedCount > 0 ? (
                  <PluginGovernanceNotice tone="neutral">
                    {`${t.admin.pluginPermissionDefaultGranted}: ${permissionPreviewSummary.defaultGrantedCount}`}
                  </PluginGovernanceNotice>
                ) : null}
                <div className="grid gap-3 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto]">
                  <div className="space-y-2">
                    <Label htmlFor="plugin-upload-permission-search">{t.common.search}</Label>
                    <Input
                      id="plugin-upload-permission-search"
                      value={permissionSearchText}
                      onChange={(event) => setPermissionSearchText(event.target.value)}
                      placeholder={t.admin.pluginPermissionSearchPlaceholder}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="plugin-upload-permission-filter">{t.common.filter}</Label>
                    <Select
                      value={permissionFilter}
                      onValueChange={(value) =>
                        setPermissionFilter(value as PermissionPreviewFilter)
                      }
                    >
                      <SelectTrigger id="plugin-upload-permission-filter">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">
                          {`${t.admin.pluginPermissionFilterAll} (${permissionPreviewItems.length})`}
                        </SelectItem>
                        <SelectItem value="required">
                          {`${t.admin.pluginPermissionRequired} (${permissionPreviewSummary.requiredCount})`}
                        </SelectItem>
                        <SelectItem value="optional">
                          {`${t.admin.pluginPermissionOptional} (${permissionPreviewSummary.optionalCount})`}
                        </SelectItem>
                        <SelectItem value="granted">
                          {`${t.admin.pluginPermissionFilterGranted} (${permissionPreviewSummary.grantedCount})`}
                        </SelectItem>
                        <SelectItem value="pending">
                          {`${t.admin.pluginPermissionFilterPending} (${pendingPermissionCount})`}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="flex flex-wrap items-end gap-2">
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={permissionPreviewSummary.optionalCount === 0 || pendingPermissionCount === 0}
                      onClick={() =>
                        setUploadGrantedPermissions((prev) =>
                          normalizeStringList([
                            ...normalizeStringList(prev),
                            ...permissionPreviewItems
                              .filter((item) => !item.required)
                              .map((item) => item.key),
                          ])
                        )
                      }
                    >
                      {t.admin.pluginPermissionGrantAllOptional}
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={recommendedGrantedPermissions.length === 0}
                      onClick={() => setUploadGrantedPermissions(recommendedGrantedPermissions)}
                    >
                      {t.admin.pluginPermissionResetRecommended}
                    </Button>
                    {(permissionSearchText.trim() || permissionFilter !== 'all') && (
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        onClick={() => {
                          setPermissionSearchText('')
                          setPermissionFilter('all')
                        }}
                      >
                        {t.common.reset}
                      </Button>
                    )}
                  </div>
                </div>
                <p className="text-xs text-muted-foreground">
                  {[
                    `${t.admin.pluginPermissionFilterAll}: ${permissionPreviewItems.length}`,
                    `${t.admin.pluginPermissionSummaryRequired}: ${permissionPreviewSummary.requiredCount}`,
                    `${t.admin.pluginPermissionSummaryOptional}: ${permissionPreviewSummary.optionalCount}`,
                    `${t.admin.pluginPermissionSummaryGranted}: ${permissionPreviewSummary.grantedCount}`,
                    `${t.admin.pluginPermissionSummaryPending}: ${pendingPermissionCount}`,
                    permissionSearchText.trim() || permissionFilter !== 'all'
                      ? `${resolvePermissionFilterLabel(permissionFilter)}: ${filteredPermissionPreviewItems.length}`
                      : null,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
                {filteredPermissionPreviewItems.length === 0 ? (
                  <PluginGovernanceNotice tone="neutral">
                    {t.admin.pluginPermissionNoMatches}
                  </PluginGovernanceNotice>
                ) : (
                  <div className="max-h-80 space-y-2 overflow-auto pr-1">
                    {filteredPermissionPreviewItems.map((item, index) => (
                      <div
                        key={`${item.key}-${index}`}
                        className={`space-y-2 rounded-md border p-3 ${
                          item.required
                            ? 'border-destructive/30 bg-destructive/5'
                            : item.checked
                              ? 'border-primary/30 bg-primary/5'
                              : 'border-input/60 bg-background'
                        }`}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0 space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="text-sm font-medium">{item.title}</span>
                              <Badge variant={item.required ? 'destructive' : 'outline'}>
                                {item.required
                                  ? t.admin.pluginPermissionRequired
                                  : t.admin.pluginPermissionOptional}
                              </Badge>
                              {!item.required && item.defaultGranted ? (
                                <Badge variant="secondary">
                                  {t.admin.pluginPermissionDefaultGranted}
                                </Badge>
                              ) : null}
                              <Badge variant={item.checked ? 'active' : 'outline'}>
                                {item.checked ? t.common.success : t.common.no}
                              </Badge>
                            </div>
                            <p className="break-all text-xs text-muted-foreground">{item.key}</p>
                          </div>
                          <label className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
                            <Checkbox
                              checked={item.checked}
                              disabled={item.required}
                              onCheckedChange={(value) => {
                                const allow = value === true
                                setUploadGrantedPermissions((prev) => {
                                  const current = normalizeStringList(prev)
                                  if (item.required) {
                                    return normalizeStringList([...current, item.key])
                                  }
                                  if (allow) {
                                    return normalizeStringList([...current, item.key])
                                  }
                                  return current.filter((permissionKey) => permissionKey !== item.key)
                                })
                              }}
                            />
                            {t.admin.pluginPermissionAllow}
                          </label>
                        </div>
                        {item.description ? (
                          <p className="text-xs text-muted-foreground">{item.description}</p>
                        ) : null}
                        {item.reason ? (
                          <p className="text-xs text-muted-foreground">{item.reason}</p>
                        ) : null}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              <PluginGovernanceNotice tone="neutral">
                {t.admin.pluginPermissionNoRequest}
              </PluginGovernanceNotice>
            )}
          </UploadSummaryCard>
            </>
          )}
          </UploadStepSection>
          <UploadStepSection
            id="plugin-upload-step-target"
            step="2"
            title={t.admin.pluginUploadStepTarget}
            description={t.admin.pluginUploadStepTargetDesc}
            status={targetStepStatus}
            statusLabel={resolveStepStatusLabel(targetStepStatus)}
            open={openSteps.target}
            onToggle={() => setOpenSteps((prev) => ({ ...prev, target: !prev.target }))}
          >
          {isMarketMode ? (
            <div className="space-y-3 rounded-md border border-input/60 bg-muted/10 p-3">
              <p className="text-xs text-muted-foreground">
                {[
                  marketTargetStatusLabel,
                  marketTargetInstalled
                    ? t.admin.pluginVersionCurrent
                    : t.admin.pluginUploadNewPlugin,
                  marketCompatibilityLabel(
                    marketCompatible,
                    marketLegacyDefaultsApplied,
                    t
                  ),
                ].join(' · ')}
              </p>
              <dl className="grid gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
                <UploadSummaryValue
                  label={t.admin.pluginName}
                  value={uploadForm.name || '-'}
                  mono
                />
                <UploadSummaryValue
                  label={t.admin.pluginUploadVersion}
                  value={uploadForm.version || '-'}
                />
                <UploadSummaryValue
                  label={t.admin.pluginVersionCurrent}
                  value={formatMarketDisplayValue((marketTargetState as any)?.current_version)}
                />
                <UploadSummaryValue
                  label={t.admin.pluginVersionStatus}
                  value={marketTargetStatusLabel}
                />
              </dl>
              {marketCompatibilityReason ? (
                <p className="text-xs text-muted-foreground">{marketCompatibilityReason}</p>
              ) : null}
            </div>
          ) : (
            <>
          <div className="grid gap-3 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
            <div className="space-y-3 rounded-md border border-input/60 bg-muted/10 p-3">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="space-y-1">
                  <p className="text-sm font-medium">{t.admin.pluginUploadTarget}</p>
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginUploadTargetHint}
                  </p>
                </div>
                <p className="text-xs text-muted-foreground">
                  {[
                    selectedTargetPlugin ? `#${selectedTargetPlugin.id}` : t.admin.pluginUploadNewPlugin,
                    selectedTargetPlugin?.version
                      ? `${t.admin.pluginVersionCurrent}: ${selectedTargetPlugin.version}`
                      : null,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
              </div>
              <div id="plugin-upload-field-target" className="space-y-3">
                <Select
                  value={uploadForm.plugin_id || 'new'}
                  onValueChange={(value) =>
                    setUploadForm((prev) => ({ ...prev, plugin_id: value === 'new' ? '' : value }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="new">{t.admin.pluginUploadNewPlugin}</SelectItem>
                    {plugins.map((plugin) => (
                      <SelectItem key={plugin.id} value={String(plugin.id)}>
                        {plugin.display_name || plugin.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  {[
                    selectedTargetPlugin
                      ? selectedTargetPlugin.display_name || selectedTargetPlugin.name
                      : t.admin.pluginUploadNewPlugin,
                    `${t.admin.pluginName}: ${uploadForm.name.trim() || '-'}`,
                    `${t.admin.pluginUploadVersion}: ${uploadForm.version.trim() || '-'}`,
                  ].join(' · ')}
                </p>
                {uploadConflictSummary.nameConflict ? (
                  <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
                    <p className="text-xs font-medium text-destructive">
                      {t.admin.pluginUploadConflictNameExists
                        .replace(
                          '{plugin}',
                          uploadConflictSummary.nameConflict.pluginDisplayName ||
                            uploadConflictSummary.nameConflict.pluginName
                        )
                        .replace('{pluginId}', String(uploadConflictSummary.nameConflict.pluginId))}
                    </p>
                    <div className="mt-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => selectTargetPlugin(uploadConflictSummary.nameConflict!.pluginId)}
                      >
                        {t.admin.pluginUploadConflictActionSelectTarget.replace(
                          '{pluginId}',
                          String(uploadConflictSummary.nameConflict.pluginId)
                        )}
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
            </div>

            <UploadSummaryCard
              title={t.admin.pluginUploadConflictCheck}
              description={t.admin.pluginUploadConflictCheckDesc}
            >
              <div className="space-y-3">
                <p className="text-xs text-muted-foreground">
                  {joinSummaryItems([
                    `${t.admin.pluginName}: ${
                      uploadConflictSummary.nameConflict ? t.common.warning : t.common.success
                    }`,
                    `${t.admin.pluginUploadManifestAdminPage}: ${adminPageConflicts.length}`,
                    `${t.admin.pluginUploadManifestUserPage}: ${userPageConflicts.length}`,
                  ])}
                </p>
                <p className="text-xs text-muted-foreground">
                  {uploadConflictSummary.hasConflict
                    ? t.admin.pluginUploadConflictResolveHint
                    : totalPageConflictCount > 0
                      ? t.admin.pluginUploadConflictCheckDesc
                      : t.admin.pluginUploadConflictNameOk}
                </p>
                {uploadConflictSummary.hasConflict ? (
                  <div className="space-y-3 rounded-md border border-destructive/30 bg-destructive/5 p-3">
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.admin.pluginUploadConflictActionTitle}</p>
                      <p className="text-xs text-muted-foreground">
                        {t.admin.pluginUploadConflictResolveHint}
                      </p>
                    </div>
                    <div className="grid gap-3">
                      {uploadConflictSummary.nameConflict ? (
                        <UploadActionCard
                          title={t.admin.pluginUploadConflictActionNameTitle}
                          description={t.admin.pluginUploadConflictActionNameDesc}
                          actionLabel={t.admin.pluginUploadConflictActionSelectTarget.replace(
                            '{pluginId}',
                            String(uploadConflictSummary.nameConflict.pluginId)
                          )}
                          onAction={() =>
                            selectTargetPlugin(uploadConflictSummary.nameConflict!.pluginId)
                          }
                          tone="danger"
                        >
                          <dl className="grid gap-3 text-sm sm:grid-cols-2">
                            <UploadSummaryValue
                              label={t.admin.pluginName}
                              value={uploadForm.name.trim() || '-'}
                              mono
                            />
                            <UploadSummaryValue
                              label={t.admin.pluginUploadTarget}
                              value={`${
                                uploadConflictSummary.nameConflict.pluginDisplayName ||
                                uploadConflictSummary.nameConflict.pluginName
                              } (#${uploadConflictSummary.nameConflict.pluginId})`}
                            />
                          </dl>
                        </UploadActionCard>
                      ) : null}
                      {adminPageConflicts.length > 0 ? (
                        <UploadActionCard
                          title={t.admin.pluginUploadConflictActionAdminPageTitle}
                          description={t.admin.pluginUploadConflictActionPageDesc}
                          actionLabel={t.admin.pluginUploadConflictActionReplacePackage}
                          onAction={reopenPackagePicker}
                          tone="danger"
                        >
                          <dl className="grid gap-3 text-sm sm:grid-cols-2">
                            <UploadSummaryValue
                              label={t.admin.pluginUploadConflictActionManifestPath}
                              value={
                                uploadConflictSummary.manifestPagePaths.adminPath ||
                                t.admin.pluginUploadManifestPageEmpty
                              }
                              mono
                            />
                            <UploadSummaryValue
                              label={t.admin.pluginUploadConflictActionConflictCount}
                              value={String(adminPageConflicts.length)}
                            />
                          </dl>
                          <div className="mt-2 space-y-1 text-xs text-muted-foreground">
                            {adminPageConflicts.map((conflict) => (
                              <p key={`admin-action-${conflict.pluginId}-${conflict.path}`}>
                                {t.admin.pluginUploadPageConflictWith
                                  .replace(
                                    '{plugin}',
                                    conflict.pluginDisplayName || conflict.pluginName
                                  )
                                  .replace('{pluginId}', String(conflict.pluginId))}
                              </p>
                            ))}
                          </div>
                        </UploadActionCard>
                      ) : null}
                      {userPageConflicts.length > 0 ? (
                        <UploadActionCard
                          title={t.admin.pluginUploadConflictActionUserPageTitle}
                          description={t.admin.pluginUploadConflictActionPageDesc}
                          actionLabel={t.admin.pluginUploadConflictActionReplacePackage}
                          onAction={reopenPackagePicker}
                          tone="danger"
                        >
                          <dl className="grid gap-3 text-sm sm:grid-cols-2">
                            <UploadSummaryValue
                              label={t.admin.pluginUploadConflictActionManifestPath}
                              value={
                                uploadConflictSummary.manifestPagePaths.userPath ||
                                t.admin.pluginUploadManifestPageEmpty
                              }
                              mono
                            />
                            <UploadSummaryValue
                              label={t.admin.pluginUploadConflictActionConflictCount}
                              value={String(userPageConflicts.length)}
                            />
                          </dl>
                          <div className="mt-2 space-y-1 text-xs text-muted-foreground">
                            {userPageConflicts.map((conflict) => (
                              <p key={`user-action-${conflict.pluginId}-${conflict.path}`}>
                                {t.admin.pluginUploadPageConflictWith
                                  .replace(
                                    '{plugin}',
                                    conflict.pluginDisplayName || conflict.pluginName
                                  )
                                  .replace('{pluginId}', String(conflict.pluginId))}
                              </p>
                            ))}
                          </div>
                        </UploadActionCard>
                      ) : null}
                    </div>
                  </div>
                ) : (
                  <div className="rounded-md border border-input/60 bg-muted/10 p-3">
                    <p className="text-sm font-medium">
                      {t.admin.pluginUploadConflictActionNoConflictTitle}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {conflictDetailsAvailable
                        ? t.admin.pluginUploadConflictActionNoConflictDesc
                        : t.admin.pluginUploadConflictNameOk}
                    </p>
                  </div>
                )}
              </div>
            </UploadSummaryCard>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>{t.admin.pluginName}</Label>
              <Input
                value={uploadForm.name}
                onChange={(e) => setUploadForm((p) => ({ ...p, name: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <Label>{t.admin.pluginDisplayName}</Label>
              <Input
                value={uploadForm.display_name}
                onChange={(e) => setUploadForm((p) => ({ ...p, display_name: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <Label>{t.admin.pluginType}</Label>
              <Input
                value={uploadForm.type}
                onChange={(e) => setUploadForm((p) => ({ ...p, type: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <Label>{t.admin.pluginRuntime}</Label>
              <Select
                value={uploadForm.runtime}
                onValueChange={(value) => setUploadForm((p) => ({ ...p, runtime: value }))}
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
              <Label>{t.admin.pluginUploadVersion}</Label>
              <Input
                value={uploadForm.version}
                onChange={(e) => setUploadForm((p) => ({ ...p, version: e.target.value }))}
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label>{runtimeAddressLabel(uploadForm.runtime, t)}</Label>
            <Input
              value={uploadForm.address}
              placeholder={runtimeAddressPlaceholder(uploadForm.runtime, t)}
              onChange={(e) => setUploadForm((p) => ({ ...p, address: e.target.value }))}
            />
            <p className="text-xs text-muted-foreground">
              {runtimeAddressHint(uploadForm.runtime, 'upload', t)}
            </p>
            <p className="text-xs text-muted-foreground">{t.admin.pluginUploadAddressSourceHint}</p>
          </div>
          <div className="space-y-2">
            <Label>{t.admin.pluginDescription}</Label>
            <Textarea
              value={uploadForm.description}
              onChange={(e) => setUploadForm((p) => ({ ...p, description: e.target.value }))}
              rows={2}
            />
          </div>
          {conflictDetailsAvailable ? (
            <details
              className="rounded-md border border-input/60 bg-muted/10 p-3"
              open={uploadConflictSummary.hasConflict}
            >
              <summary className="cursor-pointer text-sm font-medium">
                {t.admin.pluginUploadConflictDetailTitle}
              </summary>
              <p className="mt-2 text-xs text-muted-foreground">
                {t.admin.pluginUploadConflictDetailDesc}
              </p>
              <div className="mt-3 grid gap-3 lg:grid-cols-3">
                <div className="rounded-md border border-input/60 bg-background p-3">
                  <div className="min-w-0 space-y-1">
                    <p className="text-sm font-medium">{t.admin.pluginName}</p>
                    <p className="break-all text-xs text-muted-foreground">
                      {uploadForm.name.trim() || '-'}
                    </p>
                  </div>
                  <p className="mt-2 text-xs text-muted-foreground">
                    {uploadConflictSummary.nameConflict
                      ? t.admin.pluginUploadConflictNameExists
                          .replace(
                            '{plugin}',
                            uploadConflictSummary.nameConflict.pluginDisplayName ||
                              uploadConflictSummary.nameConflict.pluginName
                          )
                          .replace('{pluginId}', String(uploadConflictSummary.nameConflict.pluginId))
                      : t.admin.pluginUploadConflictNameOk}
                  </p>
                </div>

                {[
                  {
                    key: 'admin',
                    label: t.admin.pluginUploadManifestAdminPage,
                    path: uploadConflictSummary.manifestPagePaths.adminPath,
                    conflicts: adminPageConflicts,
                  },
                  {
                    key: 'user',
                    label: t.admin.pluginUploadManifestUserPage,
                    path: uploadConflictSummary.manifestPagePaths.userPath,
                    conflicts: userPageConflicts,
                  },
                ].map((item) => (
                  <div key={item.key} className="rounded-md border border-input/60 bg-background p-3">
                    <div className="min-w-0 space-y-1">
                      <p className="text-sm font-medium">{item.label}</p>
                      <p className="break-all text-xs text-muted-foreground">
                        {item.path || t.admin.pluginUploadManifestPageEmpty}
                      </p>
                    </div>
                    {item.conflicts.length > 0 ? (
                      <div className="mt-2 space-y-1 text-xs text-muted-foreground">
                        {item.conflicts.map((conflict) => (
                          <p key={`${item.key}-${conflict.pluginId}-${conflict.path}`}>
                            {t.admin.pluginUploadPageConflictWith
                              .replace(
                                '{plugin}',
                                conflict.pluginDisplayName || conflict.pluginName
                              )
                              .replace('{pluginId}', String(conflict.pluginId))}
                          </p>
                        ))}
                      </div>
                    ) : (
                      <p className="mt-2 text-xs text-muted-foreground">
                        {item.path
                          ? t.admin.pluginUploadPageAvailable
                          : t.admin.pluginUploadManifestPageEmptyHint}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            </details>
          ) : null}
            </>
          )}
          </UploadStepSection>
          {!isMarketMode ? (
            <UploadStepSection
              id="plugin-upload-step-config"
              step="3"
              title={t.admin.pluginUploadStepConfig}
              description={t.admin.pluginUploadStepConfigDesc}
              status={configStepStatus}
              statusLabel={resolveStepStatusLabel(configStepStatus)}
              open={openSteps.config}
              onToggle={() => setOpenSteps((prev) => ({ ...prev, config: !prev.config }))}
            >
          {configSchema ? (
            <PluginJSONSchemaEditor
              title={configSchema.title || t.admin.pluginConfigPresetEditor}
              description={configSchema.description || t.admin.pluginConfigPresetEditorDesc}
              schema={configSchema}
              value={uploadForm.config}
              onChange={(value) => setUploadForm((p) => ({ ...p, config: value }))}
              disabled={!configValid}
              disabledReason={!configValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined}
              t={t}
            />
          ) : null}
          <PluginJSONObjectEditor
            title={configSchema ? t.admin.pluginConfigVisualEditorExtra : t.admin.pluginConfigVisualEditor}
            description={
              configSchema
                ? t.admin.pluginConfigVisualEditorExtraDesc
                : t.admin.pluginConfigVisualEditorDesc
            }
            value={uploadForm.config}
            onChange={(value) => setUploadForm((p) => ({ ...p, config: value }))}
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
            value={uploadForm.config}
            onChange={(value) => setUploadForm((p) => ({ ...p, config: value }))}
            onBlur={onConfigBlur}
            rows={6}
            invalid={!configValid}
            invalidMessage={!configValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined}
            hints={[t.admin.pluginJsonAutoFormatHint]}
            t={t}
          />
          {runtimeParamsSchema ? (
            <PluginJSONSchemaEditor
              title={runtimeParamsSchema.title || t.admin.pluginRuntimeParamsPresetEditor}
              description={
                runtimeParamsSchema.description || t.admin.pluginRuntimeParamsPresetEditorDesc
              }
              schema={runtimeParamsSchema}
              value={uploadForm.runtime_params}
              onChange={(value) => setUploadForm((p) => ({ ...p, runtime_params: value }))}
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
            value={uploadForm.runtime_params}
            onChange={(value) => setUploadForm((p) => ({ ...p, runtime_params: value }))}
            excludedKeys={runtimeParamsSchema?.fields.map((field) => field.key) || []}
            emptyMessage={t.admin.pluginJsonObjectEditorNoExtraFields}
            disabled={!runtimeParamsValid}
            disabledReason={!runtimeParamsValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined}
            t={t}
          />
          <PluginAdvancedJsonPanel
            title={`${t.admin.pluginAdvancedJsonEditor} · ${t.admin.pluginRuntimeParams}`}
            fieldLabel={t.admin.pluginRuntimeParams}
            description={t.admin.pluginAdvancedJsonEditorDesc}
            value={uploadForm.runtime_params}
            onChange={(value) => setUploadForm((p) => ({ ...p, runtime_params: value }))}
            onBlur={onRuntimeParamsBlur}
            rows={4}
            invalid={!runtimeParamsValid}
            invalidMessage={
              !runtimeParamsValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined
            }
            hints={[t.admin.pluginRuntimeParamsVisualEditorHint, t.admin.pluginJsonAutoFormatHint]}
            t={t}
          />
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
            showPermissionEditor={false}
            disabled={!capabilitiesValid}
            disabledReason={!capabilitiesValid ? t.admin.pluginHookAccessInvalidJson : undefined}
            t={t}
          />
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
          <PluginAdvancedJsonPanel
            title={`${t.admin.pluginAdvancedJsonEditor} · ${t.admin.pluginCapabilities}`}
            fieldLabel={t.admin.pluginCapabilities}
            description={t.admin.pluginAdvancedJsonEditorDesc}
            value={uploadForm.capabilities}
            onChange={onCapabilitiesChange}
            onBlur={onCapabilitiesBlur}
            rows={8}
            invalid={!capabilitiesValid}
            invalidMessage={!capabilitiesValid ? t.admin.pluginHookAccessInvalidJson : undefined}
            hints={[t.admin.pluginCapabilitiesHint]}
            t={t}
          />
            </UploadStepSection>
          ) : null}
          <UploadStepSection
            id="plugin-upload-step-release"
            step={isMarketMode ? '3' : '4'}
            title={t.admin.pluginUploadStepRelease}
            description={t.admin.pluginUploadStepReleaseDesc}
            status={releaseStepStatus}
            statusLabel={resolveStepStatusLabel(releaseStepStatus)}
            open={openSteps.release}
            onToggle={() => setOpenSteps((prev) => ({ ...prev, release: !prev.release }))}
          >
          {!isMarketMode ? (
            <div className="space-y-2">
              <Label>{t.admin.pluginUploadChangelog}</Label>
              <Textarea
                value={uploadForm.changelog}
                onChange={(e) => setUploadForm((p) => ({ ...p, changelog: e.target.value }))}
                rows={3}
              />
            </div>
          ) : marketRelease ? (
            <div className="space-y-2">
              <Label>{t.admin.pluginUploadChangelog}</Label>
              <Textarea value={marketReleaseNotes} rows={3} disabled />
            </div>
          ) : null}
          <div className="space-y-3">
            <p className="text-xs text-muted-foreground">
              {[
                `${t.admin.pluginUploadChangelog}: ${
                  (!isMarketMode ? uploadForm.changelog : marketReleaseNotes).trim()
                    ? t.common.success
                    : t.common.noData
                }`,
                `${t.admin.pluginUploadActivate}: ${uploadForm.activate ? t.common.yes : t.common.no}`,
                `${t.admin.pluginUploadAutoStart}: ${
                  !uploadForm.activate
                    ? t.common.noData
                    : uploadForm.auto_start
                      ? t.common.yes
                      : t.common.no
                }`,
              ].join(' · ')}
            </p>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="flex items-center justify-between gap-3 rounded-md border border-input/60 bg-muted/10 p-3">
                <div className="space-y-1">
                  <Label>{t.admin.pluginUploadActivate}</Label>
                  <p className="text-xs text-muted-foreground">
                    {t.admin.pluginUploadActivationHint}
                  </p>
                </div>
                <Switch
                  checked={uploadForm.activate}
                  onCheckedChange={(checked) =>
                    setUploadForm((p) => ({
                      ...p,
                      activate: checked,
                      auto_start: checked ? p.auto_start : false,
                    }))
                  }
                />
              </div>
              <div className="flex items-center justify-between gap-3 rounded-md border border-input/60 bg-muted/10 p-3">
                <div className="space-y-1">
                  <Label>{t.admin.pluginUploadAutoStart}</Label>
                  <p className="text-xs text-muted-foreground">
                    {!uploadForm.activate
                      ? t.admin.pluginUploadAutoStartDisabledHint
                      : t.admin.pluginUploadActivationHint}
                  </p>
                </div>
                <Switch
                  checked={uploadForm.auto_start}
                  disabled={!uploadForm.activate}
                  onCheckedChange={(checked) =>
                    setUploadForm((p) => ({ ...p, auto_start: checked }))
                  }
                />
              </div>
            </div>
          </div>
          </UploadStepSection>
          {!submitReady ? (
            <div className="space-y-2 rounded-lg border border-input bg-muted/10 p-3">
              {blockedChecklistItems.map((item) => (
                <div
                  key={item.key}
                  className="rounded-md border border-input/60 bg-background p-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="min-w-0 space-y-1">
                      <p className="text-sm font-medium">{item.label}</p>
                      <p className="text-xs text-muted-foreground">{item.detail}</p>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => openAndScrollToStep(item.stepKey, item.sectionId)}
                    >
                      {t.common.view}
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t.common.cancel}
          </Button>
          <Button
            onClick={submitUpload}
            disabled={uploadPending || previewPending || !submitReady}
          >
            {uploadPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            {isMarketMode ? t.admin.pluginMarketInstall : t.admin.pluginUpload}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
