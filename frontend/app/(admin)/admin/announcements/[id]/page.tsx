'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getAdminAnnouncement, updateAnnouncement } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import toast from 'react-hot-toast'
import { ArrowLeft, Loader2, Megaphone, Save } from 'lucide-react'
import Link from 'next/link'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginSlot } from '@/components/plugins/plugin-slot'

interface AnnouncementForm {
  title: string
  content: string
  is_mandatory: boolean
  require_full_read: boolean
}

function formatAnnouncementDateTime(value?: string) {
  if (!value) {
    return '--'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

export default function EditAnnouncementPage() {
  const params = useParams()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminAnnouncementEdit)
  const announcementId = Number(params.id)

  const [form, setForm] = useState<AnnouncementForm>({
    title: '',
    content: '',
    is_mandatory: false,
    require_full_read: false,
  })
  const [baselineForm, setBaselineForm] = useState<AnnouncementForm | null>(null)

  const {
    data,
    isLoading,
    isError: announcementLoadFailed,
    refetch: refetchAnnouncement,
  } = useQuery({
    queryKey: ['adminAnnouncement', announcementId],
    queryFn: () => getAdminAnnouncement(announcementId),
    enabled: !!announcementId,
    refetchOnMount: 'always',
    staleTime: 0,
  })

  useEffect(() => {
    if (data?.data) {
      const item = data.data
      const nextForm = {
        title: item.title || '',
        content: item.content || '',
        is_mandatory: item.is_mandatory ?? false,
        require_full_read: item.require_full_read ?? false,
      }
      setForm(nextForm)
      setBaselineForm(nextForm)
    }
  }, [data])

  const saveMutation = useMutation({
    mutationFn: (formData: AnnouncementForm) => updateAnnouncement(announcementId, formData),
    onSuccess: () => {
      toast.success(t.announcement.announcementUpdated)
      router.push('/admin/announcements')
    },
    onError: (error: Error) => {
      toast.error(resolveApiErrorMessage(error, t, t.announcement.updateFailed))
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.title.trim()) {
      toast.error(t.announcement.titleRequired)
      return
    }
    saveMutation.mutate(form)
  }

  const contentCharCount = form.content.length
  const contentLineCount = form.content ? form.content.split(/\r?\n/).length : 0
  const isDirty = JSON.stringify(form) !== JSON.stringify(baselineForm)
  const adminAnnouncementDetailPluginContext = {
    view: 'admin_announcement_detail',
    resource_id: announcementId,
    form: {
      is_dirty: isDirty,
      title_length: form.title.length,
      content_char_count: contentCharCount,
      content_line_count: contentLineCount,
      is_mandatory: form.is_mandatory,
      require_full_read: form.require_full_read,
    },
    announcement: data?.data
      ? {
          id: data.data.id,
          title: data.data.title,
          category: data.data.category,
          is_mandatory: data.data.is_mandatory,
          require_full_read: data.data.require_full_read,
          created_at: data.data.created_at,
          updated_at: data.data.updated_at,
        }
      : {
          id: Number.isFinite(announcementId) ? announcementId : undefined,
        },
    state: {
      load_failed: announcementLoadFailed && !data?.data,
      not_found: !isLoading && !announcementLoadFailed && !data?.data,
    },
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <Loader2 className="mx-auto mb-4 h-8 w-8 animate-spin" />
          <p className="text-muted-foreground">{t.common.loading}</p>
        </div>
      </div>
    )
  }

  if (announcementLoadFailed && !data?.data) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <Megaphone className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.announcement.detailLoadFailed}</p>
        <p className="mb-4 mt-2 max-w-md text-sm text-muted-foreground">
          {t.announcement.detailLoadFailedDesc}
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button variant="outline" onClick={() => refetchAnnouncement()}>
            {t.common.refresh}
          </Button>
          <Button variant="ghost" asChild>
            <Link href="/admin/announcements">{t.common.back}</Link>
          </Button>
        </div>
        <PluginSlot
          slot="admin.announcement_detail.load_failed"
          context={{ ...adminAnnouncementDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  if (!data?.data) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <Megaphone className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.announcement.announcementNotFound}</p>
        <div className="mt-4 flex flex-wrap justify-center gap-2">
          <Button variant="outline" asChild>
            <Link href="/admin/announcements">
              <ArrowLeft className="mr-2 h-4 w-4" />
              {t.common.back}
            </Link>
          </Button>
        </div>
        <PluginSlot
          slot="admin.announcement_detail.not_found"
          context={{ ...adminAnnouncementDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PluginSlot
        slot="admin.announcement_detail.top"
        context={adminAnnouncementDetailPluginContext}
      />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="sm" asChild>
            <Link href="/admin/announcements">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.common.back}</span>
            </Link>
          </Button>
          <div>
            <h1 className="text-lg font-bold md:text-xl">{t.announcement.editAnnouncement}</h1>
            <p className="mt-1 text-sm text-muted-foreground">#{announcementId}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/announcements">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" form="announcement-edit-form" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.saving : t.common.save}
          </Button>
        </div>
      </div>

      <form id="announcement-edit-form" onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t.announcement.editAnnouncement}</CardTitle>
            <div className="mt-2 flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <span>#{announcementId}</span>
              {isDirty ? <span>{t.announcement.unsavedChanges}</span> : null}
              {form.is_mandatory ? <span>{t.announcement.mandatory}</span> : null}
              {form.require_full_read ? <span>{t.announcement.requireFullRead}</span> : null}
              <span>{t.announcement.contentChars.replace('{count}', String(contentCharCount))}</span>
              <span>{t.announcement.contentLines.replace('{count}', String(contentLineCount))}</span>
            </div>
            <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>
                {t.announcement.createdAt}: {formatAnnouncementDateTime(data?.data?.created_at)}
              </span>
              <span>
                {t.announcement.updatedAt}: {formatAnnouncementDateTime(data?.data?.updated_at)}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <PluginSlot
              slot="admin.announcement_detail.form.top"
              context={{ ...adminAnnouncementDetailPluginContext, section: 'form' }}
            />
            <div className="space-y-2">
              <Label htmlFor="title">
                {t.announcement.announcementTitle} <span className="text-red-500">*</span>
              </Label>
              <Input
                id="title"
                value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
                placeholder={t.announcement.announcementTitlePlaceholder}
                required
              />
            </div>

            <div className="space-y-2">
              <Label>{t.announcement.announcementContent}</Label>
              <Tabs defaultValue="edit">
                <TabsList>
                  <TabsTrigger value="edit">{t.announcement.editTab}</TabsTrigger>
                  <TabsTrigger value="preview">{t.announcement.previewTab}</TabsTrigger>
                </TabsList>
                <TabsContent value="edit">
                  <MarkdownEditor
                    value={form.content}
                    onChange={(v) => setForm({ ...form, content: v })}
                    height="400px"
                    placeholder={t.announcement.announcementContent}
                  />
                </TabsContent>
                <TabsContent value="preview">
                  <div className="min-h-[400px] rounded-md border p-4">
                    {form.content ? (
                      <MarkdownMessage content={form.content} allowHtml className="markdown-body" />
                    ) : (
                      <p className="text-muted-foreground">{t.announcement.noPreviewContent}</p>
                    )}
                  </div>
                </TabsContent>
              </Tabs>
            </div>

            <div className="flex items-center justify-between rounded-lg border p-4">
              <div className="space-y-0.5">
                <Label>{t.announcement.mandatory}</Label>
                <p className="text-sm text-muted-foreground">{t.announcement.mandatoryHint}</p>
              </div>
              <Switch
                checked={form.is_mandatory}
                onCheckedChange={(checked) =>
                  setForm({
                    ...form,
                    is_mandatory: checked,
                    require_full_read: checked ? form.require_full_read : false,
                  })
                }
              />
            </div>

            {form.is_mandatory && (
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="space-y-0.5">
                  <Label>{t.announcement.requireFullRead}</Label>
                  <p className="text-sm text-muted-foreground">
                    {t.announcement.requireFullReadHint}
                  </p>
                </div>
                <Switch
                  checked={form.require_full_read}
                  onCheckedChange={(checked) => setForm({ ...form, require_full_read: checked })}
                />
              </div>
            )}
          </CardContent>
        </Card>

        <div className="space-y-3">
          <PluginSlot
            slot="admin.announcement_detail.submit.before"
            context={{ ...adminAnnouncementDetailPluginContext, section: 'submit' }}
          />
          <div className="flex justify-end gap-4">
            <Button type="button" variant="outline" asChild>
              <Link href="/admin/announcements">{t.common.cancel}</Link>
            </Button>
            <Button type="submit" disabled={saveMutation.isPending}>
              <Save className="mr-2 h-4 w-4" />
              {saveMutation.isPending ? t.admin.saving : t.common.save}
            </Button>
          </div>
        </div>
      </form>
    </div>
  )
}
