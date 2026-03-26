'use client'

import { Children, useEffect, useMemo, useState, type ReactNode } from 'react'

import { Loader2 } from 'lucide-react'

import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import type { AdminPlugin, AdminPluginVersion } from '@/lib/api'
import type { Translations } from '@/lib/i18n'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

type VersionStatusFilter = 'all' | 'active' | 'inactive'

type PluginVersionsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  versionPlugin: AdminPlugin | null
  activateAutoStart: boolean
  setActivateAutoStart: (checked: boolean) => void
  versionsLoading: boolean
  versions: AdminPluginVersion[]
  isActivating: (pluginId: number, versionId: number) => boolean
  isDeleting: (pluginId: number, versionId: number) => boolean
  activateVersion: (pluginId: number, versionId: number, autoStart: boolean) => void
  deleteVersion: (pluginId: number, versionId: number) => void
  formatDateTime: (value?: string, locale?: string) => string
  locale: string
  t: Translations
}

function versionLifecycleBadgeVariant(
  state: string | undefined
): 'default' | 'secondary' | 'destructive' | 'outline' | 'active' {
  switch ((state || '').toLowerCase()) {
    case 'running':
      return 'active'
    case 'degraded':
      return 'destructive'
    case 'installed':
    case 'uploaded':
      return 'secondary'
    case 'retired':
      return 'outline'
    default:
      return 'outline'
  }
}

function versionLifecycleLabel(state: string | undefined, t: Translations): string {
  switch ((state || '').toLowerCase()) {
    case 'draft':
      return t.admin.pluginLifecycleDraft
    case 'uploaded':
      return t.admin.pluginLifecycleUploaded
    case 'installed':
      return t.admin.pluginLifecycleInstalled
    case 'running':
      return t.admin.pluginLifecycleRunning
    case 'paused':
      return t.admin.pluginLifecyclePaused
    case 'degraded':
      return t.admin.pluginLifecycleDegraded
    case 'retired':
      return t.admin.pluginLifecycleRetired
    default:
      return state || '-'
  }
}

function parseTimestamp(value?: string): number {
  if (!value) return 0
  const ts = new Date(value).getTime()
  return Number.isNaN(ts) ? 0 : ts
}

function buildAdminPluginVersionHostPluginSummary(plugin: AdminPlugin | null) {
  if (!plugin) {
    return null
  }
  return {
    id: plugin.id,
    name: plugin.name,
    display_name: plugin.display_name,
    type: plugin.type,
    runtime: plugin.runtime,
    version: plugin.version,
    enabled: plugin.enabled,
    lifecycle_status: plugin.lifecycle_status,
    health_status: plugin.status,
  }
}

function buildAdminPluginVersionSummary(version: AdminPluginVersion) {
  return {
    id: version.id,
    version: version.version,
    runtime: version.runtime,
    type: version.type,
    package_name: version.package_name,
    package_path: version.package_path_display || version.package_path,
    lifecycle_status: version.lifecycle_status,
    is_active: version.is_active,
    created_at: version.created_at,
    activated_at: version.activated_at,
    has_changelog: Boolean(String(version.changelog || '').trim()),
  }
}

function VersionMetaItem({
  label,
  value,
  mono,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="rounded-md border border-input/50 p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className={`mt-1 break-words font-medium ${mono ? 'font-mono text-xs' : ''}`}>{value}</p>
    </div>
  )
}

function VersionActionGroup({
  children,
  busy,
  description,
  emptyText,
  childrenClassName = 'flex flex-wrap gap-2',
  t,
}: {
  children: ReactNode
  busy?: boolean
  description?: ReactNode
  emptyText?: ReactNode
  childrenClassName?: string
  t: Translations
}) {
  const actionItems = Children.toArray(children).filter(Boolean)
  return (
    <div className="rounded-md border border-input/60 bg-muted/10 p-3">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="space-y-1">
          <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {t.admin.actions}
          </p>
          {description ? <div className="text-xs text-muted-foreground">{description}</div> : null}
        </div>
        {busy ? <span className="text-xs text-muted-foreground">{t.common.processing}</span> : null}
      </div>
      {actionItems.length > 0 ? (
        <div className={childrenClassName}>{actionItems}</div>
      ) : (
        <div className="rounded-md border border-dashed border-input/60 bg-background/80 px-3 py-2 text-xs text-muted-foreground">
          {emptyText}
        </div>
      )}
    </div>
  )
}

export function PluginVersionsDialog({
  open,
  onOpenChange,
  versionPlugin,
  activateAutoStart,
  setActivateAutoStart,
  versionsLoading,
  versions,
  isActivating,
  isDeleting,
  activateVersion,
  deleteVersion,
  formatDateTime,
  locale,
  t,
}: PluginVersionsDialogProps) {
  const [deleteCandidate, setDeleteCandidate] = useState<AdminPluginVersion | null>(null)
  const [statusFilter, setStatusFilter] = useState<VersionStatusFilter>('all')
  const [actionableOnly, setActionableOnly] = useState(false)
  const [searchText, setSearchText] = useState('')

  useEffect(() => {
    if (!open) return
    setStatusFilter('all')
    setActionableOnly(false)
    setSearchText('')
  }, [open])

  const sortedVersions = useMemo(
    () =>
      [...versions].sort((left, right) => {
        if (left.is_active !== right.is_active) {
          return left.is_active ? -1 : 1
        }
        return parseTimestamp(right.created_at) - parseTimestamp(left.created_at)
      }),
    [versions]
  )
  const activeVersion = useMemo(
    () => sortedVersions.find((item) => item.is_active) || null,
    [sortedVersions]
  )
  const latestVersion = useMemo(
    () =>
      [...versions].sort(
        (left, right) => parseTimestamp(right.created_at) - parseTimestamp(left.created_at)
      )[0] || null,
    [versions]
  )
  const busyVersionCount = useMemo(
    () =>
      sortedVersions.filter(
        (version) =>
          isActivating(version.plugin_id, version.id) || isDeleting(version.plugin_id, version.id)
      ).length,
    [isActivating, isDeleting, sortedVersions]
  )
  const pendingVersionCount = useMemo(
    () => sortedVersions.filter((version) => !version.is_active).length,
    [sortedVersions]
  )
  const normalizedSearchText = searchText.trim().toLowerCase()
  const hasActiveFilters =
    statusFilter !== 'all' || actionableOnly || normalizedSearchText !== ''
  const filteredVersions = useMemo(
    () =>
      sortedVersions.filter((version) => {
        if (statusFilter === 'active' && !version.is_active) return false
        if (statusFilter === 'inactive' && version.is_active) return false
        if (actionableOnly && version.is_active) return false
        if (!normalizedSearchText) return true
        const haystack = [
          version.version,
          version.package_name,
          version.package_path_display,
          version.package_path,
          version.runtime,
          version.changelog,
          version.lifecycle_status,
          version.type,
        ]
          .map((value) => String(value || '').toLowerCase())
          .join('\n')
        return haystack.includes(normalizedSearchText)
      }),
    [actionableOnly, normalizedSearchText, sortedVersions, statusFilter]
  )
  const pluginSummary = buildAdminPluginVersionHostPluginSummary(versionPlugin)
  const versionActionItems = filteredVersions.map((version, index) => ({
    key: String(version.id),
    slot: 'admin.plugins.version_actions',
    path: '/admin/plugins',
    hostContext: {
      view: 'admin_plugin_version_row',
      plugin: pluginSummary || undefined,
      version: buildAdminPluginVersionSummary(version),
      row: {
        index: index + 1,
      },
      filters: {
        status: statusFilter === 'all' ? undefined : statusFilter,
        actionable_only: actionableOnly || undefined,
        search: searchText.trim() || undefined,
      },
      summary: {
        total_versions: versions.length,
        filtered_versions: filteredVersions.length,
        active_version: activeVersion?.version,
        latest_version: latestVersion?.version,
      },
    },
  }))
  const versionActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/plugins',
    items: versionActionItems,
    enabled: open && filteredVersions.length > 0,
  })

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[90vh] max-w-5xl overflow-y-auto [&_.grid>*]:min-w-0">
          <DialogHeader>
            <DialogTitle>{t.admin.pluginVersionList}</DialogTitle>
            <DialogDescription>
              {versionPlugin?.display_name || versionPlugin?.name}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 rounded-lg border border-input/60 bg-muted/10 p-3">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 space-y-1">
                <p className="text-sm font-medium">
                  {activeVersion?.version || versionPlugin?.version || '-'}
                  {latestVersion ? ` · ${t.admin.pluginVersionLatestBadge}: ${latestVersion.version}` : ''}
                  {busyVersionCount > 0 ? ` · ${t.common.processing} ${busyVersionCount}` : ''}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t.admin.pluginVersionAutoStartHint}
                </p>
              </div>
              <div className="flex items-center gap-3 rounded-md border border-input/60 bg-muted/10 px-3 py-2">
                <div className="text-right">
                  <p className="text-sm font-medium">{t.admin.pluginHotUpdateAutoStart}</p>
                  <p className="text-xs text-muted-foreground">
                    {activateAutoStart ? t.common.yes : t.common.no}
                  </p>
                </div>
                <Switch checked={activateAutoStart} onCheckedChange={setActivateAutoStart} />
              </div>
            </div>
            <div className="grid gap-3 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_auto]">
              <div className="space-y-2">
                <Label htmlFor="plugin-version-search">{t.common.search}</Label>
                <Input
                  id="plugin-version-search"
                  value={searchText}
                  onChange={(event) => setSearchText(event.target.value)}
                  placeholder={t.admin.pluginVersionSearchPlaceholder}
                  disabled={versionsLoading || versions.length === 0}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="plugin-version-status-filter">
                  {t.admin.pluginVersionFilterStatus}
                </Label>
                <Select
                  value={statusFilter}
                  onValueChange={(value) => setStatusFilter(value as VersionStatusFilter)}
                  disabled={versionsLoading || versions.length === 0}
                >
                  <SelectTrigger id="plugin-version-status-filter">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t.admin.pluginVersionFilterStatusAll}</SelectItem>
                    <SelectItem value="active">
                      {t.admin.pluginVersionFilterStatusActive}
                    </SelectItem>
                    <SelectItem value="inactive">
                      {t.admin.pluginVersionFilterStatusInactive}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-wrap items-end gap-2">
                <Button
                  type="button"
                  variant={actionableOnly ? 'secondary' : 'outline'}
                  size="sm"
                  onClick={() => setActionableOnly((current) => !current)}
                  disabled={versionsLoading || pendingVersionCount === 0}
                >
                  {t.admin.pluginVersionActionableOnly}
                </Button>
                {hasActiveFilters ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setStatusFilter('all')
                      setActionableOnly(false)
                      setSearchText('')
                    }}
                  >
                    {t.common.reset}
                  </Button>
                ) : null}
              </div>
            </div>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                {statusFilter !== 'all' ? (
                  <Badge variant="secondary">
                    {t.admin.pluginVersionFilterStatus}:{' '}
                    {statusFilter === 'active'
                      ? t.admin.pluginVersionFilterStatusActive
                      : t.admin.pluginVersionFilterStatusInactive}
                  </Badge>
                ) : null}
                {actionableOnly ? (
                  <Badge variant="secondary">{t.admin.pluginVersionActionableOnly}</Badge>
                ) : null}
                {normalizedSearchText ? (
                  <Badge variant="secondary">
                    {t.common.search}: {searchText.trim()}
                  </Badge>
                ) : null}
              </div>
              <span className="text-xs text-muted-foreground">
                {filteredVersions.length}/{versions.length}
              </span>
            </div>
          </div>
          {versionsLoading ? (
            <div className="py-6 text-center text-sm text-muted-foreground">
              <Loader2 className="mr-2 inline-block h-4 w-4 animate-spin" />
              {t.common.loading}
            </div>
          ) : versions.length === 0 ? (
            <div className="py-6 text-center text-sm text-muted-foreground">
              {t.admin.pluginNoVersions}
            </div>
          ) : filteredVersions.length === 0 ? (
            <div className="space-y-3 py-6 text-center text-sm text-muted-foreground">
              <p>{t.admin.pluginVersionNoMatches}</p>
              <div className="flex flex-wrap justify-center gap-2">
                {statusFilter !== 'all' ? (
                  <Badge variant="secondary">
                    {statusFilter === 'active'
                      ? t.admin.pluginVersionFilterStatusActive
                      : t.admin.pluginVersionFilterStatusInactive}
                  </Badge>
                ) : null}
                {actionableOnly ? (
                  <Badge variant="secondary">{t.admin.pluginVersionActionableOnly}</Badge>
                ) : null}
                {normalizedSearchText ? (
                  <Badge variant="secondary">{searchText.trim()}</Badge>
                ) : null}
              </div>
              {hasActiveFilters ? (
                <div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => {
                      setStatusFilter('all')
                      setActionableOnly(false)
                      setSearchText('')
                    }}
                  >
                    {t.common.reset}
                  </Button>
                </div>
              ) : null}
            </div>
          ) : (
            <div className="space-y-3">
              {filteredVersions.map((version) => {
                const activating = isActivating(version.plugin_id, version.id)
                const deleting = isDeleting(version.plugin_id, version.id)
                const busy = activating || deleting
                const targetIsLatest = latestVersion?.id === version.id
                const rowExtensions = versionActionExtensions[String(version.id)] || []
                return (
                  <Card
                    key={version.id}
                    className={
                      version.is_active
                        ? 'border-primary/30 bg-primary/[0.02]'
                        : targetIsLatest
                          ? 'border-amber-500/20'
                          : undefined
                    }
                  >
                    <CardContent className="space-y-4 p-4">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div className="min-w-0 flex-1 space-y-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <p className="font-semibold">
                              {t.admin.pluginVersionLabel}: {version.version}
                            </p>
                            {version.is_active ? (
                              <Badge variant="active">{t.admin.pluginVersionActive}</Badge>
                            ) : null}
                            {targetIsLatest ? (
                              <Badge variant="outline">{t.admin.pluginVersionLatestBadge}</Badge>
                            ) : null}
                            <Badge variant={versionLifecycleBadgeVariant(version.lifecycle_status)}>
                              {versionLifecycleLabel(version.lifecycle_status, t)}
                            </Badge>
                            {version.runtime ? <Badge variant="outline">{version.runtime}</Badge> : null}
                          </div>
                          <p className="text-xs text-muted-foreground">
                            {version.is_active
                              ? t.admin.pluginVersionCurrentActive
                              : t.admin.pluginVersionSwitchHint
                                  .replace(
                                    '{current}',
                                    activeVersion?.version || versionPlugin?.version || '-'
                                  )
                                  .replace('{target}', version.version)}
                          </p>
                        </div>
                        <div className="flex min-w-0 flex-wrap items-center gap-2 sm:justify-end">
                          {busy ? (
                            <span className="text-xs text-muted-foreground">{t.common.processing}</span>
                          ) : null}
                        </div>
                      </div>

                      <div className="grid gap-3 text-sm md:grid-cols-3">
                        <VersionMetaItem
                          label={t.admin.createdAt}
                          value={formatDateTime(version.created_at, locale)}
                        />
                        <VersionMetaItem
                          label={t.admin.pluginVersionActivatedAt}
                          value={formatDateTime(version.activated_at, locale)}
                        />
                        <VersionMetaItem
                          label={t.admin.pluginVersionPackageName}
                          value={version.package_name || '-'}
                          mono
                        />
                      </div>

                      <VersionActionGroup
                        busy={busy}
                        t={t}
                        description={
                          version.is_active
                            ? t.admin.pluginVersionActionCurrentHint
                            : t.admin.pluginVersionSwitchHint
                                .replace('{current}', activeVersion?.version || versionPlugin?.version || '-')
                                .replace('{target}', version.version)
                        }
                        emptyText={t.admin.pluginVersionActionCurrentHint}
                        childrenClassName="grid gap-2 md:grid-cols-2 xl:grid-cols-3"
                      >
                        {!version.is_active ? (
                          <Button
                            size="sm"
                            className="justify-start"
                            onClick={() =>
                              activateVersion(version.plugin_id, version.id, activateAutoStart)
                            }
                            disabled={busy}
                          >
                            {activating ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                            {t.admin.pluginVersionActivate}
                          </Button>
                        ) : null}
                        {!version.is_active ? (
                          <Button
                            size="sm"
                            variant="destructive"
                            className="justify-start"
                            onClick={() => setDeleteCandidate(version)}
                            disabled={busy}
                          >
                            {deleting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
                            {t.common.delete}
                          </Button>
                        ) : null}
                        {rowExtensions.length > 0 ? (
                          <PluginExtensionList extensions={rowExtensions} display="inline" />
                        ) : null}
                      </VersionActionGroup>

                      <details className="rounded-md border border-input/60 bg-muted/10">
                        <summary className="cursor-pointer px-3 py-3 text-sm font-medium">
                          {t.common.detail}
                        </summary>
                        <div className="grid gap-3 border-t border-input/60 px-3 py-3 xl:grid-cols-2">
                          <div className="space-y-2 text-sm">
                            <p className="text-xs text-muted-foreground">
                              {t.admin.pluginVersionPackagePath}
                            </p>
                            <p className="break-all rounded-md border border-input/50 bg-muted/20 p-3 font-mono text-xs">
                              {version.package_path_display || version.package_path || '-'}
                            </p>
                          </div>

                          <div className="space-y-2 text-sm">
                            <p className="text-xs text-muted-foreground">
                              {t.admin.pluginUploadChangelog}
                            </p>
                            <div className="rounded-md border border-input/50 bg-muted/20 p-3">
                              {version.changelog ? (
                                <p className="break-words whitespace-pre-line">{version.changelog}</p>
                              ) : (
                                <p className="text-muted-foreground">
                                  {t.admin.pluginVersionNoChangelog}
                                </p>
                              )}
                            </div>
                          </div>
                        </div>
                      </details>
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteCandidate} onOpenChange={(open) => (!open ? setDeleteCandidate(null) : null)}>
        <AlertDialogContent className="max-w-lg">
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.pluginVersionDeleteTitle}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.admin.pluginVersionDeleteConfirm.replace('{version}', deleteCandidate?.version || '-')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {deleteCandidate ? (
            <div className="space-y-3">
              <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="destructive">{t.common.delete}</Badge>
                  <Badge variant="outline">{deleteCandidate.version}</Badge>
                  <Badge variant={versionLifecycleBadgeVariant(deleteCandidate.lifecycle_status)}>
                    {versionLifecycleLabel(deleteCandidate.lifecycle_status, t)}
                  </Badge>
                </div>
              </div>
              <div className="grid gap-3 rounded-md border border-input/60 bg-muted/10 p-3 text-sm md:grid-cols-2">
                <VersionMetaItem
                  label={t.admin.pluginVersionPackageName}
                  value={deleteCandidate.package_name || '-'}
                  mono
                />
                <VersionMetaItem
                  label={t.admin.createdAt}
                  value={formatDateTime(deleteCandidate.created_at, locale)}
                />
              </div>
            </div>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (!deleteCandidate) return
                deleteVersion(deleteCandidate.plugin_id, deleteCandidate.id)
              }}
            >
              {deleteCandidate && isDeleting(deleteCandidate.plugin_id, deleteCandidate.id) ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : null}
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
