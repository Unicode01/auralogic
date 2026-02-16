'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getApiKeys, createApiKey, deleteApiKey } from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { Key, Plus, Trash2 } from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { PERMISSIONS, PERMISSIONS_BY_CATEGORY, CATEGORY_LABEL_KEYS } from '@/lib/constants'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

export default function ApiKeysPage() {
  const [open, setOpen] = useState(false)
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminApiKeys)

  const { data, isLoading } = useQuery({
    queryKey: ['apiKeys'],
    queryFn: getApiKeys,
  })

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
        alert(`${t.admin.apiSecretOnce}: ${result.data.api_secret}`)
      }
    },
    onError: (error: any) => {
      toast.error(error.message || t.common.failed)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      toast.success(t.admin.apiKeyDeleted)
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] })
    },
  })

  function onSubmit(values: any) {
    createMutation.mutate(values)
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
        <code className="text-xs bg-muted px-2 py-1 rounded">
          {row.original.api_key}
        </code>
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
      cell: ({ row }: { row: { original: any } }) => (
        <Button
          size="sm"
          variant="destructive"
          onClick={() => {
            if (confirm(t.admin.confirmDeleteApiKey)) {
              deleteMutation.mutate(row.original.id)
            }
          }}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.admin.apiKeyManagement}</h1>
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
                    <FormItem className="flex flex-col min-h-0">
                      <FormLabel className="shrink-0">{t.admin.scopesRequired}</FormLabel>
                      <div className="flex gap-2 mb-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => form.setValue('scopes', PERMISSIONS.map(p => p.value))}
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
                            const allValues = PERMISSIONS.map(p => p.value)
                            form.setValue('scopes', allValues.filter(v => !current.includes(v)))
                          }}
                        >
                          {t.admin.permInvertSelection}
                        </Button>
                      </div>
                      <div className="border rounded-md p-4 h-64 overflow-y-auto space-y-4">
                        {Object.entries(PERMISSIONS_BY_CATEGORY).map(([category, perms]) => (
                          <div key={category} className="space-y-2">
                            <div className="font-medium text-sm text-primary border-b pb-1 flex items-center justify-between">
                              <span>{t.admin[CATEGORY_LABEL_KEYS[category] as keyof typeof t.admin] || category}</span>
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-5 px-1.5 text-xs"
                                onClick={() => {
                                  const current = form.getValues('scopes') || []
                                  const categoryValues = perms.map(p => p.value)
                                  const allSelected = categoryValues.every(v => current.includes(v))
                                  if (allSelected) {
                                    form.setValue('scopes', current.filter(v => !categoryValues.includes(v)))
                                  } else {
                                    form.setValue('scopes', [...new Set([...current, ...categoryValues])])
                                  }
                                }}
                              >
                                {(form.watch('scopes') || []).length > 0 && perms.map(p => p.value).every(v => (form.watch('scopes') || []).includes(v)) ? t.admin.permDeselectAll : t.admin.permSelectAll}
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
                                              field.onChange(currentValue.filter((v: string) => v !== permValue))
                                            }
                                          }}
                                        />
                                      </FormControl>
                                      <FormLabel className="text-sm font-normal cursor-pointer leading-tight">
                                        {t.admin[permission.labelKey as keyof typeof t.admin] || permission.value}
                                      </FormLabel>
                                    </FormItem>
                                  )}
                                />
                              ))}
                            </div>
                          </div>
                        ))}
                      </div>
                      <p className="text-xs text-muted-foreground mt-2">
                        {t.admin.selectedPermissions.replace('{count}', String(form.watch('scopes')?.length || 0))}
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

      <DataTable
        columns={columns}
        data={data?.data?.items || []}
        isLoading={isLoading}
      />
    </div>
  )
}
