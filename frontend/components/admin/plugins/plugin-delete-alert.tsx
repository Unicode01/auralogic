'use client'

import { Loader2 } from 'lucide-react'

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
import type { AdminPlugin } from '@/lib/api'
import type { Translations } from '@/lib/i18n'

type PluginDeleteAlertProps = {
  deletePlugin: AdminPlugin | null
  setDeletePlugin: (plugin: AdminPlugin | null) => void
  deletePending: boolean
  onConfirmDelete: (pluginId: number) => void
  t: Translations
}

function deleteLifecycleLabel(state: string | undefined, t: Translations): string {
  switch (state) {
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
      return state || t.common.noData
  }
}

export function PluginDeleteAlert({
  deletePlugin,
  setDeletePlugin,
  deletePending,
  onConfirmDelete,
  t,
}: PluginDeleteAlertProps) {
  return (
    <AlertDialog open={!!deletePlugin} onOpenChange={(open) => (!open ? setDeletePlugin(null) : null)}>
      <AlertDialogContent className="max-w-lg">
        <AlertDialogHeader>
          <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
          <AlertDialogDescription>{t.admin.pluginDeleteConfirm}</AlertDialogDescription>
        </AlertDialogHeader>
        {deletePlugin ? (
          <div className="space-y-3">
            <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="destructive">{t.common.delete}</Badge>
                <Badge variant="outline">#{deletePlugin.id}</Badge>
                <Badge variant="outline">{deletePlugin.version || t.common.noData}</Badge>
                <Badge variant="outline">{deletePlugin.runtime || t.common.noData}</Badge>
                <Badge variant="outline">
                  {deleteLifecycleLabel(deletePlugin.lifecycle_status, t)}
                </Badge>
              </div>
              <p className="mt-2 break-words font-medium">
                {deletePlugin.display_name || deletePlugin.name}
              </p>
              {deletePlugin.display_name && deletePlugin.display_name !== deletePlugin.name ? (
                <p className="mt-1 break-all text-xs text-muted-foreground">{deletePlugin.name}</p>
              ) : null}
            </div>

            {deletePlugin.display_name && deletePlugin.display_name !== deletePlugin.name ? (
              <div className="space-y-2 rounded-md border border-input/60 bg-muted/10 p-3 text-sm">
                <div>
                  <p className="text-xs text-muted-foreground">{t.admin.pluginName}</p>
                  <p className="mt-1 break-all font-mono text-xs">{deletePlugin.name}</p>
                </div>
              </div>
            ) : null}
          </div>
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
          <AlertDialogAction
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            onClick={() => {
              if (deletePlugin) onConfirmDelete(deletePlugin.id)
            }}
            disabled={deletePending || !deletePlugin}
          >
            {deletePending ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : null}
            {t.common.delete}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
