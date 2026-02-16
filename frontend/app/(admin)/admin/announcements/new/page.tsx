'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useMutation } from '@tanstack/react-query'
import { createAnnouncement } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from '@/components/ui/tabs'
import toast from 'react-hot-toast'
import { ArrowLeft, Save } from 'lucide-react'
import Link from 'next/link'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { MarkdownMessage } from '@/components/ui/markdown-message'

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

  const saveMutation = useMutation({
    mutationFn: (data: AnnouncementForm) => createAnnouncement(data),
    onSuccess: () => {
      toast.success(t.announcement.announcementCreated)
      router.push('/admin/announcements')
    },
    onError: (error: Error) => {
      toast.error(`${t.announcement.createFailed}: ${error.message}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.title.trim()) {
      toast.error(`${t.announcement.announcementTitle} is required`)
      return
    }
    saveMutation.mutate(form)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="sm" asChild>
          <Link href="/admin/announcements">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.common.back}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold">
          {t.announcement.addAnnouncement}
        </h1>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t.announcement.addAnnouncement}</CardTitle>
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
                  <Textarea
                    value={form.content}
                    onChange={(e) => setForm({ ...form, content: e.target.value })}
                    className="min-h-[400px] font-mono"
                    placeholder={t.announcement.announcementContent}
                  />
                </TabsContent>
                <TabsContent value="preview">
                  <div className="min-h-[400px] border rounded-md p-4">
                    {form.content ? (
                      <MarkdownMessage
                        content={form.content}
                        allowHtml
                        className="prose dark:prose-invert max-w-none text-base [&_*]:text-foreground"
                      />
                    ) : (
                      <p className="text-muted-foreground">
                        {t.announcement.noPreviewContent}
                      </p>
                    )}
                  </div>
                </TabsContent>
              </Tabs>
            </div>

            <div className="flex items-center justify-between rounded-lg border p-4">
              <div className="space-y-0.5">
                <Label>{t.announcement.mandatory}</Label>
                <p className="text-sm text-muted-foreground">
                  {t.announcement.mandatoryHint}
                </p>
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
                  onCheckedChange={(checked) =>
                    setForm({ ...form, require_full_read: checked })
                  }
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
