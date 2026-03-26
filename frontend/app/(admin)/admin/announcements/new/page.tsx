'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation } from '@tanstack/react-query'
import { createAnnouncement } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import toast from 'react-hot-toast'
import { ArrowLeft, Save } from 'lucide-react'
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

export default function CreateAnnouncementPage() {
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminAnnouncementNew)

  const [form, setForm] = useState<AnnouncementForm>({
    title: '',
    content: '',
    is_mandatory: false,
    require_full_read: false,
  })
  const emptyForm: AnnouncementForm = {
    title: '',
    content: '',
    is_mandatory: false,
    require_full_read: false,
  }

  const saveMutation = useMutation({
    mutationFn: (data: AnnouncementForm) => createAnnouncement(data),
    onSuccess: () => {
      toast.success(t.announcement.announcementCreated)
      router.push('/admin/announcements')
    },
    onError: (error: Error) => {
      toast.error(resolveApiErrorMessage(error, t, t.announcement.createFailed))
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
  const isDirty = JSON.stringify(form) !== JSON.stringify(emptyForm)
  const adminAnnouncementNewPluginContext = {
    view: 'admin_announcement_new',
    form: {
      is_dirty: isDirty,
      title_length: form.title.length,
      content_char_count: contentCharCount,
      content_line_count: contentLineCount,
      is_mandatory: form.is_mandatory,
      require_full_read: form.require_full_read,
    },
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.announcement_new.top" context={adminAnnouncementNewPluginContext} />
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="sm" asChild>
            <Link href="/admin/announcements">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.common.back}</span>
            </Link>
          </Button>
          <div>
            <h1 className="text-lg font-bold md:text-xl">{t.announcement.addAnnouncement}</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {t.announcement.announcementDetail}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/announcements">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" form="announcement-create-form" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.creating : t.common.save}
          </Button>
        </div>
      </div>

      <form id="announcement-create-form" onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t.announcement.addAnnouncement}</CardTitle>
            <p className="text-xs text-muted-foreground">
              {t.announcement.editorHint}
              {' · '}
              {t.announcement.contentChars.replace('{count}', String(contentCharCount))}
              {' · '}
              {t.announcement.contentLines.replace('{count}', String(contentLineCount))}
            </p>
          </CardHeader>
          <CardContent className="space-y-4">
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

        <div className="flex justify-end gap-4">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/announcements">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.creating : t.common.save}
          </Button>
        </div>
      </form>
    </div>
  )
}
