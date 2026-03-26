'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getApiKeys, createApiKey, deleteApiKey } from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { DataTable } from '@/components/admin/data-table'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import { Checkbox } from '@/components/ui/checkbox'
import { useForm } from 'react-hook-form'
import { useToast } from '@/hooks/use-toast'
import { Copy, Plus, Trash2 } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { PERMISSIONS, PERMISSIONS_BY_CATEGORY, CATEGORY_LABEL_KEYS } from '@/lib/constants'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

interface ApiKeyItem {
  id: number
  key_name: string
  api_key: string
  platform?: string
  scopes?: string[]
  rate_limit?: number
  created_at?: string
}

function buildAdminApiKeySummary(apiKey: ApiKeyItem) {
  return {
    id: apiKey.id,
    key_name: apiKey.key_name,
    api_key: apiKey.api_key,
    platform: apiKey.platform,
    scopes: Array.isArray(apiKey.scopes) ? apiKey.scopes : [],
    scopes_count: Array.isArray(apiKey.scopes) ? apiKey.scopes.length : 0,
    rate_limit: apiKey.rate_limit,
    created_at: apiKey.created_at,
  }
}

export default function ApiKeysPage() {
  const [open, setOpen] = useState(false)
  const [secretDialogOpen, setSecretDialogOpen] = useState(false)
  const [newSecret, setNewSecret] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<any>(null)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminApiKeys)
  const getAdminLabel = (key: string, fallback: string) => {
    const value = t.admin[key as keyof typeof t.admin]
    return typeof value === 'string' ? value : fallback
  }

  const { data, isLoading } = useQuery({
    queryKey: ['apiKeys'],
    queryFn: getApiKeys,
  })
  const apiKeys: ApiKeyItem[] = data?.data?.items || []

  const form = useForm({
    defaultValues: {
      key_name: '',
      platform: '',
      scopes: [] as string[],
      rate_limit: 1000,
    },
  })

  const createMutation = useMutation({
    mutationFn: createApiKey,
    onSuccess: (result) => {
      toast.success(t.admin.apiKeyCreated)
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      setOpen(false)
      form.reset()

      if (result?.data?.api_secret) {
        setNewSecret(result.data.api_secret)
        setSecretDialogOpen(true)
      }
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.common.failed))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      toast.success(t.admin.apiKeyDeleted)
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
      setDeleteTarget(null)
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.common.failed))
    },
  })

  function onSubmit(values: any) {
    createMutation.mutate(values)
  }

  const adminApiKeysPluginContext = {
    view: 'admin_api_keys',
    summary: {
      total_keys: apiKeys.length,
      create_dialog_open: open,
      secret_dialog_open: secretDialogOpen,
      selected_scope_count: form.watch('scopes')?.length || 0,
    },
  }
  const adminApiKeyRowActionItems = apiKeys.map((apiKey: ApiKeyItem, index: number) => ({
    key: String(apiKey.id),
    slot: 'admin.api_keys.row_actions',
    path: '/admin/api-keys',
    hostContext: {
      view: 'admin_api_keys_row',
      api_key: buildAdminApiKeySummary(apiKey),
      row: {
        index: index + 1,
      },
      summary: adminApiKeysPluginContext.summary,
    },
  }))
  const adminApiKeyRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/api-keys',
    items: adminApiKeyRowActionItems,
    enabled: adminApiKeyRowActionItems.length > 0,
  })
  const adminApiKeyCreatePluginContext = {
    view: 'admin_api_key_create',
    form: {
      key_name: form.watch('key_name') || undefined,
      platform: form.watch('platform') || undefined,
      rate_limit: form.watch('rate_limit') || undefined,
      selected_scopes: form.watch('scopes') || [],
    },
    summary: adminApiKeysPluginContext.summary,
  }

  const columns = [
    {
      header: 'ID',
      accessorKey: 'id',
    },
    {
      header: t.admin.name,
      accessorKey: 'key_name',
    },
    {
      header: 'API Key',
      cell: ({ row }: { row: { original: any } }) => (
        <code className="rounded bg-muted px-2 py-1 text-xs">{row.original.api_key}</code>
      ),
    },
    {
      header: t.admin.platform,
      accessorKey: 'platform',
    },
    {
      header: t.admin.rateLimit,
      cell: ({ row }: { row: { original: any } }) => (
        <span>{t.admin.rateLimitDisplay.replace('{count}', String(row.original.rate_limit))}</span>
      ),
    },
    {
      header: t.admin.createdAt,
      cell: ({ row }: { row: { original: any } }) =>
        row.original.created_at ? formatDate(row.original.created_at) : '-',
    },
    {
      header: t.admin.actions,
      cell: ({ row }: { row: { original: ApiKeyItem } }) => {
        const rowExtensions = adminApiKeyRowActionExtensions[String(row.original.id)] || []
        return (
          <div className="flex items-center gap-2">
            <Button size="sm" variant="destructive" onClick={() => setDeleteTarget(row.original)}>
              <Trash2 className="h-4 w-4" />
            </Button>
            {rowExtensions.length > 0 ? (
              <PluginExtensionList extensions={rowExtensions} display="inline" />
            ) : null}
          </div>
        )
      },
    },
  ]

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.api_keys.top" context={adminApiKeysPluginContext} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.apiKeyManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {apiKeys.length > 0 ? t.admin.apiSecretOnce : t.admin.createApiKey}
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              {t.admin.createApiKey}
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t.admin.createApiKey}</DialogTitle>
            </DialogHeader>
            <PluginSlot slot="admin.api_keys.create.top" context={adminApiKeyCreatePluginContext} />
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                <FormField
                  control={form.control}
                  name="key_name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t.admin.keyName}</FormLabel>
                      <FormControl>
                        <Input placeholder={t.admin.keyNamePlaceholder} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="platform"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t.admin.platformId}</FormLabel>
                      <FormControl>
                        <Input placeholder={t.admin.platformIdPlaceholder} {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="rate_limit"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t.admin.rateLimit}</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          {...field}
                          onChange={(e) => field.onChange(parseInt(e.target.value))}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="scopes"
                  render={() => (
                    <FormItem className="flex min-h-0 flex-col">
                      <FormLabel className="shrink-0">{t.admin.scopesRequired}</FormLabel>
                      <div className="mb-2 flex gap-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() =>
                            form.setValue(
                              'scopes',
                              PERMISSIONS.map((p) => p.value)
                            )
                          }
                        >
                          {t.admin.permSelectAll}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => form.setValue('scopes', [])}
                        >
                          {t.admin.permDeselectAll}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            const current = form.getValues('scopes') || []
                            const allValues = PERMISSIONS.map((p) => p.value)
                            form.setValue(
                              'scopes',
                              allValues.filter((v) => !current.includes(v))
                            )
                          }}
                        >
                          {t.admin.permInvertSelection}
                        </Button>
                      </div>
                      <div className="h-64 space-y-4 overflow-y-auto rounded-md border p-4">
                        {Object.entries(PERMISSIONS_BY_CATEGORY).map(([category, perms]) => (
                          <div key={category} className="space-y-2">
                            <div className="flex items-center justify-between border-b pb-1 text-sm font-medium text-primary">
                              <span>{getAdminLabel(CATEGORY_LABEL_KEYS[category], category)}</span>
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-5 px-1.5 text-xs"
                                onClick={() => {
                                  const current = form.getValues('scopes') || []
                                  const categoryValues = perms.map((p) => p.value)
                                  const allSelected = categoryValues.every((v) =>
                                    current.includes(v)
                                  )
                                  if (allSelected) {
                                    form.setValue(
                                      'scopes',
                                      current.filter((v) => !categoryValues.includes(v))
                                    )
                                  } else {
                                    form.setValue('scopes', [
                                      ...new Set([...current, ...categoryValues]),
                                    ])
                                  }
                                }}
                              >
                                {(form.watch('scopes') || []).length > 0 &&
                                perms
                                  .map((p) => p.value)
                                  .every((v) => (form.watch('scopes') || []).includes(v))
                                  ? t.admin.permDeselectAll
                                  : t.admin.permSelectAll}
                              </Button>
                            </div>
                            <div className="grid grid-cols-2 gap-2 pl-2">
                              {perms.map((permission) => (
                                <FormField
                                  key={permission.value}
                                  control={form.control}
                                  name="scopes"
                                  render={({ field }) => (
                                    <FormItem className="flex items-start space-x-2 space-y-0">
                                      <FormControl>
                                        <Checkbox
                                          checked={field.value?.includes(permission.value)}
                                          onCheckedChange={(checked) => {
                                            const currentValue = field.value || []
                                            const permValue = permission.value
                                            if (checked) {
                                              field.onChange([...currentValue, permValue])
                                            } else {
                                              field.onChange(
                                                currentValue.filter((v: string) => v !== permValue)
                                              )
                                            }
                                          }}
                                        />
                                      </FormControl>
                                      <FormLabel className="cursor-pointer text-sm font-normal leading-tight">
                                        {getAdminLabel(permission.labelKey, permission.value)}
                                      </FormLabel>
                                    </FormItem>
                                  )}
                                />
                              ))}
                            </div>
                          </div>
                        ))}
                      </div>
                      <p className="mt-2 text-xs text-muted-foreground">
                        {t.admin.selectedPermissions.replace(
                          '{count}',
                          String(form.watch('scopes')?.length || 0)
                        )}
                      </p>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <div className="flex gap-2">
                  <Button type="submit" disabled={createMutation.isPending}>
                    {createMutation.isPending ? t.admin.creating : t.common.create}
                  </Button>
                  <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                    {t.common.cancel}
                  </Button>
                </div>
              </form>
            </Form>
          </DialogContent>
        </Dialog>
      </div>

      <DataTable columns={columns} data={apiKeys} isLoading={isLoading} />

      {/* Secret Key Dialog */}
      <Dialog
        open={secretDialogOpen}
        onOpenChange={(open) => {
          if (!open) {
            setSecretDialogOpen(false)
            setNewSecret('')
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t.admin.apiSecretOnce}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">{t.admin.apiSecretOnce}</p>
            <code className="block select-all break-all rounded-md bg-muted p-3 font-mono text-sm">
              {newSecret}
            </code>
          </div>
          <div className="flex justify-end">
            <Button
              variant="outline"
              onClick={async () => {
                if (typeof navigator === 'undefined' || !navigator.clipboard) {
                  return
                }
                await navigator.clipboard.writeText(newSecret)
                toast.success(t.common.copiedToClipboard)
              }}
            >
              <Copy className="mr-2 h-4 w-4" />
              {t.common.copy}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>{t.admin.confirmDeleteApiKey}</p>
                {deleteTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-medium">
                      {deleteTarget.key_name || deleteTarget.api_key}
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-muted-foreground">
                      <span>{deleteTarget.platform || '-'}</span>
                      <span>{deleteTarget.api_key}</span>
                      <span>
                        {t.admin.permissions}: {deleteTarget.scopes?.length || 0}
                      </span>
                    </div>
                  </div>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteTarget) {
                  deleteMutation.mutate(deleteTarget.id)
                }
              }}
              className="bg-red-600 hover:bg-red-700"
            >
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
