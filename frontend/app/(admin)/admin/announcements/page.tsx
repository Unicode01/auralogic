'use client'

import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createAnnouncement,
  deleteAnnouncement,
  getAdminAnnouncement,
  getAdminAnnouncements,
  type Announcement,
  updateAnnouncement,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
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
import toast from 'react-hot-toast'
import { FileText, Loader2, Megaphone, Pencil, Plus, Save, Trash2, X } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

interface AnnouncementForm {
  title: string
  content: string
  is_mandatory: boolean
  require_full_read: boolean
}

type EditorMode = 'empty' | 'create' | 'edit'

function createEmptyAnnouncementForm(): AnnouncementForm {
  return {
    title: '',
    content: '',
    is_mandatory: false,
    require_full_read: false,
  }
}

function buildAnnouncementPayload(form: AnnouncementForm) {
  return {
    title: form.title,
    content: form.content,
    category: 'general' as const,
    send_email: false,
    send_sms: false,
    is_mandatory: form.is_mandatory,
    require_full_read: form.require_full_read,
  }
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

function buildAdminAnnouncementRowSummary(item: Announcement) {
  return {
    id: item.id,
    title: item.title,
    category: item.category,
    is_mandatory: item.is_mandatory,
    require_full_read: item.require_full_read,
    created_at: item.created_at,
    updated_at: item.updated_at,
    content_length: item.content?.length || 0,
    content_preview: item.content?.replace(/\s+/g, ' ').trim().slice(0, 120) || '',
  }
}

export default function AdminAnnouncementsPage() {
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminAnnouncements)

  const [page, setPage] = useState(1)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const limit = 20

  const [editorMode, setEditorMode] = useState<EditorMode>('empty')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [form, setForm] = useState<AnnouncementForm>(createEmptyAnnouncementForm())
  const [baselineForm, setBaselineForm] = useState<AnnouncementForm | null>(null)

  const {
    data,
    isLoading,
    isError: announcementsLoadFailed,
    refetch: refetchAnnouncements,
  } = useQuery({
    queryKey: ['adminAnnouncements', page],
    queryFn: () => getAdminAnnouncements({ page, limit }),
  })

  const announcements: Announcement[] = data?.data?.items || []
  const total = data?.data?.total || 0
  const totalPages = Math.ceil(total / limit) || 1

  const {
    data: detailData,
    isLoading: detailLoading,
    isError: detailLoadFailed,
    refetch: refetchAnnouncementDetail,
  } = useQuery({
    queryKey: ['adminAnnouncement', selectedId],
    queryFn: () => getAdminAnnouncement(selectedId as number),
    enabled: editorMode === 'edit' && !!selectedId,
    refetchOnMount: 'always',
    staleTime: 0,
  })

  useEffect(() => {
    if (editorMode !== 'edit') return
    if (!detailData?.data) return
    const item = detailData.data
    const nextForm = {
      title: item.title || '',
      content: item.content || '',
      is_mandatory: item.is_mandatory ?? false,
      require_full_read: item.require_full_read ?? false,
    }
    setForm(nextForm)
    setBaselineForm(nextForm)
  }, [editorMode, detailData])

  const deleteMutation = useMutation({
    mutationFn: deleteAnnouncement,
    onSuccess: (_res, id) => {
      toast.success(t.announcement.announcementDeleted)
      queryClient.invalidateQueries({ queryKey: ['adminAnnouncements'] })
      if (typeof id === 'number' && id === selectedId) {
        setEditorMode('empty')
        setSelectedId(null)
        setForm(createEmptyAnnouncementForm())
        setBaselineForm(null)
      }
      setDeleteId(null)
    },
    onError: (error: Error) => {
      toast.error(resolveApiErrorMessage(error, t, t.announcement.deleteFailed))
    },
  })

  const createMutation = useMutation({
    mutationFn: (payload: AnnouncementForm) => createAnnouncement(payload),
    onSuccess: (res: any) => {
      toast.success(t.announcement.announcementCreated)
      queryClient.invalidateQueries({ queryKey: ['adminAnnouncements'] })
      const maybeId = res?.data?.id
      if (typeof maybeId === 'number' && maybeId > 0) {
        setEditorMode('edit')
        setSelectedId(maybeId)
        queryClient.invalidateQueries({ queryKey: ['adminAnnouncement', maybeId] })
        return
      }
      setEditorMode('empty')
      setSelectedId(null)
      setForm(createEmptyAnnouncementForm())
      setBaselineForm(null)
    },
    onError: (error: Error) => {
      toast.error(resolveApiErrorMessage(error, t, t.announcement.createFailed))
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: AnnouncementForm }) =>
      updateAnnouncement(id, payload),
    onSuccess: () => {
      toast.success(t.announcement.announcementUpdated)
      queryClient.invalidateQueries({ queryKey: ['adminAnnouncements'] })
      setBaselineForm(form)
      if (selectedId) {
        queryClient.invalidateQueries({ queryKey: ['adminAnnouncement', selectedId] })
      }
    },
    onError: (error: Error) => {
      toast.error(resolveApiErrorMessage(error, t, t.announcement.updateFailed))
    },
  })

  const isSaving = createMutation.isPending || updateMutation.isPending
  const isEditorLoading = editorMode === 'edit' && detailLoading
  const selectedAnnouncement = detailData?.data as Announcement | undefined
  const editorNotFound =
    editorMode === 'edit' && !isEditorLoading && !detailLoadFailed && !selectedAnnouncement
  const currentPageMandatoryCount = announcements.filter((item) => item.is_mandatory).length
  const visibleStart = total === 0 ? 0 : (page - 1) * limit + 1
  const visibleEnd = total === 0 ? 0 : Math.min(page * limit, total)
  const rangeSummary = t.announcement.rangeSummary
    .replace('{start}', String(visibleStart))
    .replace('{end}', String(visibleEnd))
    .replace('{total}', String(total))
  const contentCharCount = form.content.length
  const contentLineCount = form.content ? form.content.split(/\r?\n/).length : 0
  const hasUnsavedChanges =
    editorMode !== 'empty' && JSON.stringify(form) !== JSON.stringify(baselineForm)
  const adminAnnouncementsPluginContext = {
    view: 'admin_announcements',
    pagination: {
      page,
      total,
      total_pages: totalPages,
      limit,
    },
    editor: {
      mode: editorMode,
      selected_id: selectedId || undefined,
      has_unsaved_changes: hasUnsavedChanges,
    },
    summary: {
      current_page_count: announcements.length,
      mandatory_count: currentPageMandatoryCount,
      content_char_count: contentCharCount,
      content_line_count: contentLineCount,
    },
    state: {
      list_load_failed: announcementsLoadFailed && announcements.length === 0,
      list_empty: !isLoading && !announcementsLoadFailed && announcements.length === 0,
      editor_empty: editorMode === 'empty',
      editor_loading: isEditorLoading,
      editor_load_failed: editorMode === 'edit' && detailLoadFailed && !selectedAnnouncement,
      editor_not_found: editorNotFound,
    },
  }
  const adminAnnouncementRowActionItems = announcements.map((item, index) => ({
    key: String(item.id),
    slot: 'admin.announcements.row_actions',
    path: '/admin/announcements',
    hostContext: {
      view: 'admin_announcements_row',
      announcement: buildAdminAnnouncementRowSummary(item),
      row: {
        index: index + 1,
        absolute_index: (page - 1) * limit + index + 1,
        selected: editorMode === 'edit' && selectedId === item.id,
      },
      pagination: adminAnnouncementsPluginContext.pagination,
      editor: adminAnnouncementsPluginContext.editor,
      summary: adminAnnouncementsPluginContext.summary,
    },
  }))
  const adminAnnouncementRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/announcements',
    items: adminAnnouncementRowActionItems,
    enabled: announcements.length > 0,
  })
  const deleteTarget = deleteId
    ? announcements.find((item) => item.id === deleteId) ||
      (selectedId === deleteId ? selectedAnnouncement : undefined)
    : undefined

  const handleStartCreate = () => {
    const nextForm = createEmptyAnnouncementForm()
    setEditorMode('create')
    setSelectedId(null)
    setForm(nextForm)
    setBaselineForm(nextForm)
  }

  const handleSelectAnnouncement = (id: number) => {
    setEditorMode('edit')
    setSelectedId(id)
  }

  const handleCloseEditor = () => {
    setEditorMode('empty')
    setSelectedId(null)
    setForm(createEmptyAnnouncementForm())
    setBaselineForm(null)
  }

  const handleSave = () => {
    if (!form.title.trim()) {
      toast.error(t.announcement.titleRequired)
      return
    }
    if (editorMode === 'create') {
      createMutation.mutate(buildAnnouncementPayload(form))
      return
    }
    if (editorMode === 'edit' && selectedId) {
      updateMutation.mutate({ id: selectedId, payload: buildAnnouncementPayload(form) })
    }
  }

  return (
    <div className="flex min-h-[calc(100dvh-4rem)] flex-col gap-4">
      <PluginSlot slot="admin.announcements.top" context={adminAnnouncementsPluginContext} />
      <div className="flex shrink-0 flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-lg font-bold md:text-xl">{t.admin.announcementManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{rangeSummary}</p>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-6 xl:grid-cols-[420px_1fr]">
        <Card className="flex min-h-0 min-w-0 flex-col overflow-hidden">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="text-base">{t.announcement.announcements}</CardTitle>
              <Button size="sm" onClick={handleStartCreate}>
                <Plus className="mr-1.5 h-4 w-4" />
                {t.announcement.addAnnouncement}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="min-h-0 flex-1 p-0">
            <ScrollArea className="h-full">
              <div className="p-3">
                {isLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-6 w-6 animate-spin" />
                  </div>
                ) : announcementsLoadFailed && announcements.length === 0 ? (
                  <div className="space-y-4 py-12 text-center">
                    <Megaphone className="mx-auto h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.announcement.loadFailed}</p>
                      <p className="text-xs text-muted-foreground">{t.announcement.loadFailedDesc}</p>
                    </div>
                    <div className="flex justify-center">
                      <Button size="sm" variant="outline" onClick={() => refetchAnnouncements()}>
                        {t.common.refresh}
                      </Button>
                    </div>
                    <PluginSlot
                      slot="admin.announcements.load_failed"
                      context={{ ...adminAnnouncementsPluginContext, section: 'list_state' }}
                    />
                  </div>
                ) : announcements.length === 0 ? (
                  <div className="py-12 text-center text-muted-foreground">
                    <Megaphone className="mx-auto mb-3 h-10 w-10 opacity-50" />
                    {t.announcement.noAnnouncements}
                    <PluginSlot
                      slot="admin.announcements.empty"
                      context={{ ...adminAnnouncementsPluginContext, section: 'list_state' }}
                    />
                  </div>
                ) : (
                  <div className="space-y-1">
                    {announcements.map((item) => (
                      <div
                        key={item.id}
                        className={`group flex items-center justify-between gap-3 rounded-md px-3 py-2.5 transition-colors hover:bg-muted/50 ${
                          editorMode === 'edit' && selectedId === item.id ? 'bg-muted/60' : ''
                        }`}
                        role="button"
                        tabIndex={0}
                        onClick={() => handleSelectAnnouncement(item.id)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            handleSelectAnnouncement(item.id)
                          }
                        }}
                      >
                        <div className="min-w-0 flex-1">
                          <div className="truncate font-medium">{item.title}</div>
                          <div className="mt-1 flex items-center gap-2">
                            {item.is_mandatory && (
                              <Badge variant="destructive" className="text-xs">
                                {t.announcement.mandatory}
                              </Badge>
                            )}
                            {item.require_full_read && (
                              <Badge variant="secondary" className="text-xs">
                                {t.announcement.requireFullRead}
                              </Badge>
                            )}
                            <span className="text-xs text-muted-foreground">
                              {new Date(item.created_at).toLocaleDateString()}
                            </span>
                          </div>
                          {item.content ? (
                            <div className="mt-1 truncate text-xs text-muted-foreground">
                              {item.content.replace(/\s+/g, ' ').trim()}
                            </div>
                          ) : null}
                        </div>
                        <div className="flex shrink-0 items-center gap-1">
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleSelectAnnouncement(item.id)
                            }}
                            aria-label={t.announcement.editAnnouncement}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="text-destructive hover:text-destructive"
                            onClick={(e) => {
                              e.stopPropagation()
                              setDeleteId(item.id)
                            }}
                            aria-label={t.common.delete}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                          <PluginExtensionList
                            extensions={
                              adminAnnouncementRowActionExtensions[String(item.id)] || []
                            }
                            display="inline"
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {totalPages > 1 && (
                  <div className="mt-4 flex items-center justify-between gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={page <= 1}
                      onClick={() => setPage((p) => p - 1)}
                    >
                      {t.admin.prevPage}
                    </Button>
                    <span className="text-xs text-muted-foreground">
                      {t.admin.page
                        .replace('{current}', page.toString())
                        .replace('{total}', totalPages.toString())}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={page >= totalPages}
                      onClick={() => setPage((p) => p + 1)}
                    >
                      {t.admin.nextPage}
                    </Button>
                  </div>
                )}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>

        <Card className="flex min-h-0 min-w-0 flex-col overflow-hidden">
          {editorMode === 'empty' ? (
            <CardContent className="flex-1 p-0">
              <div className="flex h-full flex-col items-center justify-center px-4 py-10 text-center">
                <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-muted">
                  <FileText className="h-6 w-6 text-muted-foreground" />
                </div>
                <div className="text-base font-semibold">{t.announcement.announcements}</div>
                <div className="mt-1 text-sm text-muted-foreground">
                  {t.announcement.announcementDetail}
                </div>
                <div className="mt-6 space-y-3">
                  <PluginSlot
                    slot="admin.announcements.editor.empty"
                    context={{ ...adminAnnouncementsPluginContext, section: 'editor_state' }}
                  />
                  <Button onClick={handleStartCreate}>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t.announcement.addAnnouncement}
                  </Button>
                </div>
              </div>
            </CardContent>
          ) : (
            <>
              <CardHeader className="min-w-0 pb-3">
                <div className="flex items-start justify-between gap-4">
                    <div className="space-y-2">
                      <CardTitle className="text-base">
                        {editorMode === 'create'
                          ? t.announcement.addAnnouncement
                          : t.announcement.editAnnouncement}
                      </CardTitle>
                      <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                      {editorMode === 'edit' ? (
                        <>
                          {selectedId ? <span>#{selectedId}</span> : null}
                          <span>
                            {t.announcement.createdAt}:{' '}
                            {formatAnnouncementDateTime(selectedAnnouncement?.created_at)}
                          </span>
                          <span>
                            {t.announcement.updatedAt}:{' '}
                            {formatAnnouncementDateTime(selectedAnnouncement?.updated_at)}
                          </span>
                          {hasUnsavedChanges ? <span>{t.announcement.unsavedChanges}</span> : null}
                        </>
                      ) : (
                        <>
                          <span>{t.announcement.editorHint}</span>
                          {hasUnsavedChanges ? <span>{t.announcement.unsavedChanges}</span> : null}
                        </>
                      )}
                     </div>
                      <PluginSlot
                        slot="admin.announcements.editor.meta.after"
                        context={{ ...adminAnnouncementsPluginContext, section: 'editor_meta' }}
                      />
                    </div>
                  <div className="flex items-center gap-2">
                    <Button type="button" variant="outline" size="sm" onClick={handleCloseEditor}>
                      <X className="mr-1.5 h-4 w-4" />
                      {t.common.cancel}
                    </Button>
                    {!isEditorLoading ? (
                      <Button type="button" size="sm" disabled={isSaving} onClick={handleSave}>
                        {isSaving ? (
                          <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                        ) : (
                          <Save className="mr-1.5 h-4 w-4" />
                        )}
                        {t.common.save}
                      </Button>
                    ) : null}
                  </div>
                </div>
              </CardHeader>
              <CardContent className="min-h-0 min-w-0 flex-1 p-0">
                {isEditorLoading ? (
                  <div className="flex h-full items-center justify-center">
                    <Loader2 className="h-8 w-8 animate-spin" />
                  </div>
                ) : detailLoadFailed && !selectedAnnouncement ? (
                  <div className="flex h-full flex-col items-center justify-center gap-4 px-4 py-10 text-center">
                    <Megaphone className="h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.announcement.detailLoadFailed}</p>
                      <p className="text-xs text-muted-foreground">
                        {t.announcement.detailLoadFailedDesc}
                      </p>
                    </div>
                    <div className="flex flex-wrap justify-center gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => refetchAnnouncementDetail()}
                      >
                        {t.common.refresh}
                      </Button>
                      <Button type="button" size="sm" variant="ghost" onClick={handleCloseEditor}>
                        {t.common.cancel}
                      </Button>
                    </div>
                  </div>
                ) : editorNotFound ? (
                  <div className="flex h-full flex-col items-center justify-center gap-4 px-4 py-10 text-center">
                    <FileText className="h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.announcement.announcementNotFound}</p>
                      <p className="text-xs text-muted-foreground">{t.announcement.announcementDetail}</p>
                    </div>
                    <Button type="button" size="sm" variant="outline" onClick={handleCloseEditor}>
                      {t.common.back}
                    </Button>
                  </div>
                ) : (
                  <div className="flex h-full min-h-0 min-w-0 flex-col gap-4 p-4">
                    <div className="space-y-2">
                      <Label htmlFor="announcementTitle">
                        {t.announcement.announcementTitle} <span className="text-red-500">*</span>
                      </Label>
                      <Input
                        id="announcementTitle"
                        value={form.title}
                        onChange={(e) => setForm({ ...form, title: e.target.value })}
                        placeholder={t.announcement.announcementTitlePlaceholder}
                        required
                      />
                    </div>

                    <div className="grid gap-3 md:grid-cols-2">
                      <div className="flex items-center justify-between rounded-lg border p-3">
                        <div className="space-y-0.5">
                          <Label>{t.announcement.mandatory}</Label>
                          <p className="text-xs text-muted-foreground">
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
                      <div
                        className={`flex items-center justify-between rounded-lg border p-3 ${
                          form.is_mandatory ? '' : 'opacity-60'
                        }`}
                      >
                        <div className="space-y-0.5">
                          <Label>{t.announcement.requireFullRead}</Label>
                          <p className="text-xs text-muted-foreground">
                            {t.announcement.requireFullReadHint}
                          </p>
                        </div>
                        <Switch
                          checked={form.require_full_read}
                          disabled={!form.is_mandatory}
                          onCheckedChange={(checked) =>
                            setForm({ ...form, require_full_read: checked })
                          }
                        />
                      </div>
                    </div>

                    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
                      <Tabs
                        defaultValue="edit"
                        className="flex min-h-0 w-full min-w-0 flex-1 flex-col"
                      >
                        <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                          <div className="space-y-1">
                            <Label>{t.announcement.announcementContent}</Label>
                            <div className="text-xs text-muted-foreground">
                              {t.announcement.contentChars.replace(
                                '{count}',
                                String(contentCharCount)
                              )}
                              {' · '}
                              {t.announcement.contentLines.replace(
                                '{count}',
                                String(contentLineCount)
                              )}
                            </div>
                          </div>
                          <TabsList className="shrink-0">
                            <TabsTrigger value="edit">{t.announcement.editTab}</TabsTrigger>
                            <TabsTrigger value="preview">{t.announcement.previewTab}</TabsTrigger>
                          </TabsList>
                        </div>

                        <div className="min-h-0 min-w-0 flex-1">
                          <TabsContent value="edit" className="mt-0 h-full min-h-0 min-w-0">
                            <MarkdownEditor
                              value={form.content}
                              onChange={(v) => setForm({ ...form, content: v })}
                              fill
                              className="h-full min-h-0 w-full"
                              placeholder={t.announcement.announcementContent}
                            />
                          </TabsContent>
                          <TabsContent value="preview" className="mt-0 h-full min-h-0 min-w-0">
                            <div className="h-full min-h-0 w-full overflow-auto rounded-md border bg-background p-4">
                              {form.content ? (
                                <MarkdownMessage
                                  content={form.content}
                                  allowHtml
                                  className="markdown-body"
                                />
                              ) : (
                                <p className="text-muted-foreground">
                                  {t.announcement.noPreviewContent}
                                </p>
                              )}
                            </div>
                          </TabsContent>
                        </div>
                      </Tabs>
                    </div>
                  </div>
                )}
              </CardContent>
            </>
          )}
        </Card>
      </div>

      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>{t.announcement.confirmDelete}</p>
                {deleteTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-medium">{deleteTarget.title}</div>
                    <div className="mt-1 text-muted-foreground">
                      {t.announcement.createdAt}:{' '}
                      {formatAnnouncementDateTime(deleteTarget.created_at)}
                      {deleteTarget.is_mandatory ? ` · ${t.announcement.mandatory}` : ''}
                      {deleteTarget.require_full_read ? ` · ${t.announcement.requireFullRead}` : ''}
                    </div>
                  </div>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
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
