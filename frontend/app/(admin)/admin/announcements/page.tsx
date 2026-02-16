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
import { Textarea } from '@/components/ui/textarea'
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
import {
  FileText,
  Loader2,
  Megaphone,
  Pencil,
  Plus,
  Save,
  Trash2,
  X,
} from 'lucide-react'
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

type EditorMode = 'empty' | 'create' | 'edit'

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
  const [form, setForm] = useState<AnnouncementForm>({
    title: '',
    content: '',
    is_mandatory: false,
    require_full_read: false,
  })

  const { data, isLoading } = useQuery({
    queryKey: ['adminAnnouncements', page],
    queryFn: () => getAdminAnnouncements({ page, limit }),
  })

  const announcements: Announcement[] = data?.data?.items || []
  const total = data?.data?.total || 0
  const totalPages = Math.ceil(total / limit) || 1

  const { data: detailData, isLoading: detailLoading } = useQuery({
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
    setForm({
      title: item.title || '',
      content: item.content || '',
      is_mandatory: item.is_mandatory ?? false,
      require_full_read: item.require_full_read ?? false,
    })
  }, [editorMode, detailData])

  const deleteMutation = useMutation({
    mutationFn: deleteAnnouncement,
    onSuccess: (_res, id) => {
      toast.success(t.announcement.announcementDeleted)
      queryClient.invalidateQueries({ queryKey: ['adminAnnouncements'] })
      if (typeof id === 'number' && id === selectedId) {
        setEditorMode('empty')
        setSelectedId(null)
        setForm({
          title: '',
          content: '',
          is_mandatory: false,
          require_full_read: false,
        })
      }
      setDeleteId(null)
    },
    onError: (error: Error) => {
      toast.error(`${t.announcement.deleteFailed}: ${error.message}`)
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
    },
    onError: (error: Error) => {
      toast.error(`${t.announcement.createFailed}: ${error.message}`)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: AnnouncementForm }) =>
      updateAnnouncement(id, payload),
    onSuccess: () => {
      toast.success(t.announcement.announcementUpdated)
      queryClient.invalidateQueries({ queryKey: ['adminAnnouncements'] })
      if (selectedId) {
        queryClient.invalidateQueries({ queryKey: ['adminAnnouncement', selectedId] })
      }
    },
    onError: (error: Error) => {
      toast.error(`${t.announcement.updateFailed}: ${error.message}`)
    },
  })

  const isSaving = createMutation.isPending || updateMutation.isPending
  const isEditorLoading = editorMode === 'edit' && detailLoading

  const handleStartCreate = () => {
    setEditorMode('create')
    setSelectedId(null)
    setForm({
      title: '',
      content: '',
      is_mandatory: false,
      require_full_read: false,
    })
  }

  const handleSelectAnnouncement = (id: number) => {
    setEditorMode('edit')
    setSelectedId(id)
  }

  const handleCloseEditor = () => {
    setEditorMode('empty')
    setSelectedId(null)
    setForm({
      title: '',
      content: '',
      is_mandatory: false,
      require_full_read: false,
    })
  }

  const handleSave = () => {
    if (!form.title.trim()) {
      toast.error(`${t.announcement.announcementTitle} is required`)
      return
    }
    if (editorMode === 'create') {
      createMutation.mutate(form)
      return
    }
    if (editorMode === 'edit' && selectedId) {
      updateMutation.mutate({ id: selectedId, payload: form })
    }
  }

  return (
    <div className="min-h-[calc(100dvh-4rem)] flex flex-col gap-4">
      <div className="flex items-start justify-between gap-4 shrink-0">
        <h1 className="text-lg md:text-xl font-bold">
          {t.admin.announcementManagement}
        </h1>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[420px_1fr] gap-6 flex-1 min-h-0">
        <Card className="overflow-hidden flex flex-col min-w-0 min-h-0">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="text-base">
                {t.announcement.announcements}
              </CardTitle>
              <Button size="sm" onClick={handleStartCreate}>
                <Plus className="mr-1.5 h-4 w-4" />
                {t.announcement.addAnnouncement}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="p-0 flex-1 min-h-0">
            <ScrollArea className="h-full">
              <div className="p-3">
                {isLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-6 w-6 animate-spin" />
                  </div>
                ) : announcements.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Megaphone className="h-10 w-10 mx-auto mb-3 opacity-50" />
                    {t.announcement.noAnnouncements}
                  </div>
                ) : (
                  <div className="space-y-1">
                    {announcements.map((item) => (
                      <div
                        key={item.id}
                        className={`group flex items-center justify-between gap-3 rounded-md px-3 py-2.5 hover:bg-muted/50 transition-colors ${
                          editorMode === 'edit' && selectedId === item.id
                            ? 'bg-muted/60'
                            : ''
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
                        <div className="flex-1 min-w-0">
                          <div className="font-medium truncate">{item.title}</div>
                          <div className="flex items-center gap-2 mt-1">
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
                        </div>
                        <div className="flex items-center gap-1 shrink-0">
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
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {totalPages > 1 && (
                  <div className="flex items-center justify-between gap-2 mt-4">
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

        <Card className="overflow-hidden flex flex-col min-w-0 min-h-0">
          {editorMode === 'empty' ? (
            <CardContent className="p-0 flex-1">
              <div className="flex flex-col items-center justify-center text-center px-4 py-10 h-full">
                <div className="h-12 w-12 rounded-xl bg-muted flex items-center justify-center mb-4">
                  <FileText className="h-6 w-6 text-muted-foreground" />
                </div>
                <div className="text-base font-semibold">
                  {t.announcement.announcements}
                </div>
                <div className="text-sm text-muted-foreground mt-1">
                  {t.announcement.announcementDetail}
                </div>
                <div className="mt-6">
                  <Button onClick={handleStartCreate}>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t.announcement.addAnnouncement}
                  </Button>
                </div>
              </div>
            </CardContent>
          ) : (
            <>
              <CardHeader className="pb-3 min-w-0">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <CardTitle className="text-base">
                      {editorMode === 'create'
                        ? t.announcement.addAnnouncement
                        : t.announcement.editAnnouncement}
                    </CardTitle>
                    <div className="text-xs text-muted-foreground mt-1">
                      {editorMode === 'edit' && selectedId
                        ? `#${selectedId}`
                        : t.announcement.announcementDetail}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={handleCloseEditor}
                    >
                      <X className="mr-1.5 h-4 w-4" />
                      {t.common.cancel}
                    </Button>
                    {!isEditorLoading ? (
                      <Button
                        type="button"
                        size="sm"
                        disabled={isSaving}
                        onClick={handleSave}
                      >
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
              <CardContent className="p-0 flex-1 min-h-0 min-w-0">
                {isEditorLoading ? (
                  <div className="h-full flex items-center justify-center">
                    <Loader2 className="h-8 w-8 animate-spin" />
                  </div>
                ) : (
                  <div className="p-4 flex flex-col gap-4 h-full min-h-0 min-w-0">
                    <div className="space-y-2">
                      <Label htmlFor="announcementTitle">
                        {t.announcement.announcementTitle}{' '}
                        <span className="text-red-500">*</span>
                      </Label>
                      <Input
                        id="announcementTitle"
                        value={form.title}
                        onChange={(e) =>
                          setForm({ ...form, title: e.target.value })
                        }
                        placeholder={t.announcement.announcementTitlePlaceholder}
                        required
                      />
                    </div>

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
                            require_full_read: checked
                              ? form.require_full_read
                              : false,
                          })
                        }
                      />
                    </div>

                    {form.is_mandatory ? (
                      <div className="flex items-center justify-between rounded-lg border p-3">
                        <div className="space-y-0.5">
                          <Label>{t.announcement.requireFullRead}</Label>
                          <p className="text-xs text-muted-foreground">
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
                    ) : null}

                    <div className="flex flex-col gap-2 flex-1 min-h-0 min-w-0">
                      <Tabs
                        defaultValue="edit"
                        className="flex flex-col flex-1 min-h-0 w-full min-w-0"
                      >
                        <div className="flex items-center justify-between gap-3 mb-2">
                          <Label>{t.announcement.announcementContent}</Label>
                          <TabsList className="shrink-0">
                            <TabsTrigger value="edit">
                              {t.announcement.editTab}
                            </TabsTrigger>
                            <TabsTrigger value="preview">
                              {t.announcement.previewTab}
                            </TabsTrigger>
                          </TabsList>
                        </div>

                        <div className="flex-1 min-h-0 min-w-0">
                          <TabsContent
                            value="edit"
                            className="mt-0 h-full min-h-0 min-w-0"
                          >
                            <Textarea
                              value={form.content}
                              onChange={(e) =>
                                setForm({ ...form, content: e.target.value })
                              }
                              className="h-full min-h-0 w-full font-mono resize-none"
                              placeholder={t.announcement.announcementContent}
                            />
                          </TabsContent>
                          <TabsContent
                            value="preview"
                            className="mt-0 h-full min-h-0 min-w-0"
                          >
                            <div className="h-full min-h-0 w-full overflow-auto border rounded-md p-4 bg-background">
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

      <AlertDialog
        open={deleteId !== null}
        onOpenChange={() => setDeleteId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.announcement.confirmDelete}
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

