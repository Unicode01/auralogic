'use client'

import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getMarketingBatch,
  getMarketingBatches,
  getMarketingUsers,
  type MarketingBatchItem,
  previewAdminMarketing,
  type PreviewAdminMarketingResult,
  sendAdminMarketing,
  type SendAdminMarketingData,
  type SendAdminMarketingResult,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import { Loader2, Mail, MessageSquare, Search, Send, Users } from 'lucide-react'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { usePermission } from '@/hooks/use-permission'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'

type RecipientMode = 'all' | 'selected'
type TriState = 'all' | 'true' | 'false'

interface AdminUserItem {
  id: number
  name?: string
  email?: string
  phone?: string | null
  role?: string
  is_active?: boolean
  email_verified?: boolean
  locale?: string
  country?: string
  email_notify_marketing?: boolean
  sms_notify_marketing?: boolean
}

const EMPTY_MARKETING_USERS: AdminUserItem[] = []
const EMPTY_MARKETING_BATCHES: MarketingBatchItem[] = []

function formatDateTime(dateString?: string, locale?: string) {
  if (!dateString) return '-'
  const date = new Date(dateString)
  if (Number.isNaN(date.getTime())) return dateString
  return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })
}

function buildAdminMarketingRecipientSummary(user: AdminUserItem) {
  return {
    id: user.id,
    name: user.name,
    email: user.email,
    phone: user.phone,
    role: user.role,
    is_active: user.is_active,
    email_verified: user.email_verified,
    locale: user.locale,
    country: user.country,
    email_notify_marketing: user.email_notify_marketing,
    sms_notify_marketing: user.sms_notify_marketing,
  }
}

function buildAdminMarketingBatchSummary(batch: MarketingBatchItem) {
  return {
    id: batch.id,
    batch_no: batch.batch_no,
    title: batch.title,
    status: batch.status,
    total_tasks: batch.total_tasks,
    processed_tasks: batch.processed_tasks,
    send_email: batch.send_email,
    send_sms: batch.send_sms,
    target_all: batch.target_all,
    requested_user_count: batch.requested_user_count,
    targeted_users: batch.targeted_users,
    email_sent: batch.email_sent,
    email_failed: batch.email_failed,
    email_skipped: batch.email_skipped,
    sms_sent: batch.sms_sent,
    sms_failed: batch.sms_failed,
    sms_skipped: batch.sms_skipped,
    operator_id: batch.operator_id,
    operator_name: batch.operator_name,
    started_at: batch.started_at,
    completed_at: batch.completed_at,
    failed_reason: batch.failed_reason,
    created_at: batch.created_at,
    updated_at: batch.updated_at,
  }
}

export default function AdminMarketingPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()
  const { hasPermission } = usePermission()
  const [permissionReady, setPermissionReady] = useState(false)
  const canViewMarketing = permissionReady && hasPermission('marketing.view')
  const canSendMarketing = permissionReady && hasPermission('marketing.send')
  const canViewRecipientUsers =
    permissionReady && hasPermission('marketing.view') && hasPermission('user.view')
  usePageTitle(t.pageTitle.adminMarketing)
  const formatMarketingError = (error: unknown, fallback: string) => {
    const detail = resolveApiErrorMessage(error, t, fallback)
    return detail === fallback ? fallback : `${fallback}: ${detail}`
  }

  useEffect(() => {
    setPermissionReady(true)
  }, [])

  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [contentTab, setContentTab] = useState<'edit' | 'preview'>('edit')
  const [previewTitle, setPreviewTitle] = useState('')
  const [previewContent, setPreviewContent] = useState('')
  const [sendEmail, setSendEmail] = useState(true)
  const [sendSms, setSendSms] = useState(false)
  const [recipientMode, setRecipientMode] = useState<RecipientMode>('all')
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([])
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [userStatusFilter, setUserStatusFilter] = useState<TriState>('all')
  const [userVerifiedFilter, setUserVerifiedFilter] = useState<TriState>('all')
  const [userEmailMarketingFilter, setUserEmailMarketingFilter] = useState<TriState>('all')
  const [userSmsMarketingFilter, setUserSmsMarketingFilter] = useState<TriState>('all')
  const [userHasPhoneFilter, setUserHasPhoneFilter] = useState<TriState>('all')
  const [userLocaleFilter, setUserLocaleFilter] = useState('')
  const [userPage, setUserPage] = useState(1)
  const [batchPage, setBatchPage] = useState(1)
  const [lastBatchId, setLastBatchId] = useState<number | null>(null)
  const [lastResult, setLastResult] = useState<SendAdminMarketingResult | null>(null)

  const parseTriState = (value: TriState): boolean | undefined => {
    if (value === 'all') return undefined
    return value === 'true'
  }
  const cleanFilterLabel = (label: string) =>
    label.replace(/^筛选[:：]\s*/u, '').replace(/^Filter:\s*/u, '')

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setPreviewTitle(title)
      setPreviewContent(content)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [title, content])

  useEffect(() => {
    if (recipientMode !== 'selected') return
    const timer = window.setTimeout(() => {
      setUserPage(1)
      setSearch(searchInput.trim())
    }, 250)
    return () => window.clearTimeout(timer)
  }, [searchInput, recipientMode])

  useEffect(() => {
    if (!permissionReady) return
    if (!canViewRecipientUsers && recipientMode === 'selected') {
      setRecipientMode('all')
    }
  }, [permissionReady, canViewRecipientUsers, recipientMode])

  const usersQuery = useQuery({
    queryKey: [
      'marketingUsers',
      userPage,
      search,
      userStatusFilter,
      userVerifiedFilter,
      userEmailMarketingFilter,
      userSmsMarketingFilter,
      userHasPhoneFilter,
      userLocaleFilter,
    ],
    queryFn: () =>
      getMarketingUsers({
        page: userPage,
        limit: 20,
        search: search || undefined,
        is_active: parseTriState(userStatusFilter),
        email_verified: parseTriState(userVerifiedFilter),
        email_notify_marketing: parseTriState(userEmailMarketingFilter),
        sms_notify_marketing: parseTriState(userSmsMarketingFilter),
        has_phone: parseTriState(userHasPhoneFilter),
        locale: userLocaleFilter || undefined,
      }),
    enabled: canViewRecipientUsers && recipientMode === 'selected',
  })

  const users: AdminUserItem[] = usersQuery.data?.data?.items ?? EMPTY_MARKETING_USERS
  const userPagination = usersQuery.data?.data?.pagination
  const totalUserPages = Math.max(userPagination?.total_pages || 1, 1)

  useEffect(() => {
    if (recipientMode !== 'selected') return
    if (userPage > totalUserPages) {
      setUserPage(totalUserPages)
    }
  }, [recipientMode, userPage, totalUserPages])

  const previewUserId = useMemo(() => {
    if (recipientMode === 'selected' && selectedUserIds.length > 0) {
      return selectedUserIds[0]
    }
    return undefined
  }, [recipientMode, selectedUserIds])

  const previewQuery = useQuery({
    queryKey: ['marketingPreview', previewTitle, previewContent, previewUserId],
    queryFn: () =>
      previewAdminMarketing({
        title: previewTitle.trim(),
        content: previewContent,
        user_id: previewUserId,
      }),
    enabled:
      canSendMarketing &&
      contentTab === 'preview' &&
      (previewTitle.trim().length > 0 || previewContent.trim().length > 0),
  })

  const previewData = previewQuery.data?.data as PreviewAdminMarketingResult | undefined

  const batchesQuery = useQuery({
    queryKey: ['marketingBatches', batchPage],
    queryFn: () => getMarketingBatches({ page: batchPage, limit: 8 }),
    enabled: canViewMarketing,
  })

  const batches: MarketingBatchItem[] = batchesQuery.data?.data?.items ?? EMPTY_MARKETING_BATCHES
  const batchPagination = batchesQuery.data?.data?.pagination
  const totalBatchPages = batchPagination?.total_pages || 1

  useEffect(() => {
    if (!lastBatchId && batches.length > 0) {
      setLastBatchId(batches[0].id)
    }
  }, [batches, lastBatchId])

  const batchDetailQuery = useQuery({
    queryKey: ['marketingBatchDetail', lastBatchId],
    queryFn: () => getMarketingBatch(lastBatchId as number),
    enabled: canViewMarketing && !!lastBatchId,
    refetchInterval: (query) => {
      const status = (query.state.data as any)?.data?.status
      return status === 'queued' || status === 'running' ? 2000 : false
    },
  })

  const selectableCurrentPageIds = useMemo(
    () => users.filter((u) => u.is_active !== false).map((u) => u.id),
    [users]
  )
  const selectedSet = useMemo(() => new Set(selectedUserIds), [selectedUserIds])
  const allCurrentPageSelected =
    selectableCurrentPageIds.length > 0 &&
    selectableCurrentPageIds.every((id) => selectedSet.has(id))

  const sendMutation = useMutation({
    mutationFn: (payload: SendAdminMarketingData) => sendAdminMarketing(payload),
    onSuccess: (res: any) => {
      const data = (res?.data || null) as (SendAdminMarketingResult & { id?: number }) | null
      setLastResult(data)
      const nextBatchId = data?.id || data?.batch_id
      if (nextBatchId) {
        setLastBatchId(nextBatchId)
      }
      setBatchPage(1)
      queryClient.invalidateQueries({ queryKey: ['marketingBatchDetail'] })
      queryClient.invalidateQueries({ queryKey: ['marketingBatches'] })
      toast.success(t.admin.marketingQueuedSuccess || t.admin.marketingSentSuccess)
    },
    onError: (error: unknown) => {
      toast.error(formatMarketingError(error, t.admin.marketingSendFailed))
    },
  })

  const activeBatch =
    (batchDetailQuery.data?.data as MarketingBatchItem | undefined) ||
    (lastResult as unknown as MarketingBatchItem | null)
  const activeBatchStatus = activeBatch?.status
  const activeBatchProgress =
    activeBatch && typeof activeBatch.total_tasks === 'number' && activeBatch.total_tasks > 0
      ? `${activeBatch.processed_tasks}/${activeBatch.total_tasks}`
      : '-'
  const marketingActiveFilterCount =
    Number(Boolean(search.trim())) +
    Number(userStatusFilter !== 'all') +
    Number(userVerifiedFilter !== 'all') +
    Number(userEmailMarketingFilter !== 'all') +
    Number(userSmsMarketingFilter !== 'all') +
    Number(userHasPhoneFilter !== 'all') +
    Number(Boolean(userLocaleFilter))
  const adminMarketingPluginContext = {
    view: 'admin_marketing',
    permissions: {
      can_view: canViewMarketing,
      can_send: canSendMarketing,
      can_view_recipient_users: canViewRecipientUsers,
    },
    message: {
      title: title || undefined,
      content_length: content.length,
      send_email: sendEmail,
      send_sms: sendSms,
      recipient_mode: recipientMode,
    },
    filters: {
      search: search || undefined,
      is_active: parseTriState(userStatusFilter),
      email_verified: parseTriState(userVerifiedFilter),
      email_notify_marketing: parseTriState(userEmailMarketingFilter),
      sms_notify_marketing: parseTriState(userSmsMarketingFilter),
      has_phone: parseTriState(userHasPhoneFilter),
      locale: userLocaleFilter || undefined,
    },
    pagination: {
      user_page: userPage,
      user_total_pages: totalUserPages,
      batch_page: batchPage,
      batch_total_pages: totalBatchPages,
    },
    selection: {
      selected_user_count: selectedUserIds.length,
      selected_user_ids: selectedUserIds.slice(0, 20),
    },
    batch: {
      active_batch_id: lastBatchId || undefined,
      active_batch_status: activeBatchStatus || undefined,
      active_batch_progress: activeBatchProgress,
    },
    summary: {
      active_filter_count: marketingActiveFilterCount,
      preview_ready: Boolean(previewData),
    },
  }
  const adminMarketingRecipientActionItems =
    recipientMode === 'selected'
      ? users.map((user, index) => ({
          key: String(user.id),
          slot: 'admin.marketing.recipient_row_actions',
          path: '/admin/marketing',
          hostContext: {
            view: 'admin_marketing_recipient_row',
            user: buildAdminMarketingRecipientSummary(user),
            row: {
              index: index + 1,
              selected: selectedSet.has(user.id),
              selectable: user.is_active !== false,
            },
            filters: adminMarketingPluginContext.filters,
            selection: adminMarketingPluginContext.selection,
            summary: adminMarketingPluginContext.summary,
          },
        }))
      : []
  const adminMarketingRecipientActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/marketing',
    items: adminMarketingRecipientActionItems,
    enabled: adminMarketingRecipientActionItems.length > 0,
  })
  const adminMarketingBatchActionItems = batches.map((batch, index) => ({
    key: String(batch.id),
    slot: 'admin.marketing.batch_row_actions',
    path: '/admin/marketing',
    hostContext: {
      view: 'admin_marketing_batch_row',
      batch: buildAdminMarketingBatchSummary(batch),
      row: {
        index: index + 1,
        selected: lastBatchId === batch.id,
      },
      summary: {
        active_batch_id: lastBatchId || undefined,
        active_batch_status: activeBatchStatus || undefined,
      },
    },
  }))
  const adminMarketingBatchActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/marketing',
    items: adminMarketingBatchActionItems,
    enabled: adminMarketingBatchActionItems.length > 0,
  })

  const getBatchStatusText = (status?: string) => {
    switch (status) {
      case 'queued':
        return t.admin.marketingStatusQueued
      case 'running':
        return t.admin.marketingStatusRunning
      case 'completed':
        return t.admin.marketingStatusCompleted
      case 'failed':
        return t.admin.marketingStatusFailed
      default:
        return status || '-'
    }
  }

  const toggleUser = (userId: number, checked: boolean) => {
    setSelectedUserIds((prev) => {
      if (checked) {
        if (prev.includes(userId)) return prev
        return [...prev, userId]
      }
      return prev.filter((id) => id !== userId)
    })
  }

  const handleToggleCurrentPage = () => {
    if (allCurrentPageSelected) {
      setSelectedUserIds((prev) => prev.filter((id) => !selectableCurrentPageIds.includes(id)))
      return
    }
    setSelectedUserIds((prev) => Array.from(new Set([...prev, ...selectableCurrentPageIds])))
  }

  const handleSend = () => {
    if (!permissionReady) {
      return
    }
    if (!canSendMarketing) {
      toast.error(t.admin.marketingNoSendPermission)
      return
    }
    if (!title.trim()) {
      toast.error(t.admin.marketingTitleRequired)
      return
    }
    if (!content.trim()) {
      toast.error(t.admin.marketingContentRequired)
      return
    }
    if (!sendEmail && !sendSms) {
      toast.error(t.admin.marketingChannelRequired)
      return
    }
    if (recipientMode === 'selected' && !canViewRecipientUsers) {
      toast.error(t.message.noPermission)
      return
    }
    if (recipientMode === 'selected' && selectedUserIds.length === 0) {
      toast.error(t.admin.marketingRecipientRequired)
      return
    }

    const payload: SendAdminMarketingData = {
      title: title.trim(),
      content: content.trim(),
      send_email: sendEmail,
      send_sms: sendSms,
      target_all: recipientMode === 'all',
    }
    if (recipientMode === 'selected') {
      payload.user_ids = selectedUserIds
    }

    sendMutation.mutate(payload)
  }

  if (!permissionReady) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold md:text-3xl">{t.admin.marketingManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t.admin.marketingDescription}</p>
        </div>
        <Card className="h-fit self-start">
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {t.common.loading}
          </CardContent>
        </Card>
      </div>
    )
  }

  if (!canViewMarketing) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold md:text-3xl">{t.admin.marketingManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t.admin.marketingDescription}</p>
        </div>
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {t.admin.marketingNoViewPermission}
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.marketing.top" context={adminMarketingPluginContext} />
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-2xl font-bold md:text-3xl">{t.admin.marketingManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t.admin.marketingDescription}</p>
          {!canSendMarketing ? (
            <p className="mt-2 text-xs text-amber-600">{t.admin.marketingNoSendPermission}</p>
          ) : null}
        </div>
        <Button onClick={handleSend} disabled={sendMutation.isPending || !canSendMarketing}>
          {sendMutation.isPending ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <Send className="mr-2 h-4 w-4" />
          )}
          {sendMutation.isPending ? t.admin.marketingSending : t.admin.marketingSendNow}
        </Button>
      </div>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <Card className="self-start">
          <CardHeader>
            <CardTitle>{t.admin.marketingMessage}</CardTitle>
            <CardDescription>{t.admin.marketingMessageDesc}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="marketing-title">{t.admin.marketingTitle}</Label>
              <Input
                id="marketing-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t.admin.marketingTitlePlaceholder}
              />
            </div>

            <div className="space-y-2">
              <Tabs
                value={contentTab}
                onValueChange={(value) => setContentTab(value as 'edit' | 'preview')}
              >
                <div className="flex items-center justify-between gap-3">
                  <Label>{t.admin.marketingContent}</Label>
                  <TabsList className="shrink-0">
                    <TabsTrigger value="edit">{t.admin.marketingContentEdit}</TabsTrigger>
                    <TabsTrigger value="preview">{t.admin.marketingContentPreview}</TabsTrigger>
                  </TabsList>
                </div>

                <TabsContent value="edit" className="mt-2">
                  <MarkdownEditor
                    value={content}
                    onChange={setContent}
                    height="300px"
                    placeholder={t.admin.marketingContentPlaceholder}
                  />
                </TabsContent>

                <TabsContent value="preview" className="mt-2 space-y-3">
                  {previewQuery.isLoading || previewQuery.isFetching ? (
                    <div className="flex items-center justify-center rounded-lg border p-6 text-sm text-muted-foreground">
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t.admin.marketingPreviewLoading}
                    </div>
                  ) : previewData ? (
                    <>
                      {sendEmail ? (
                        <div className="overflow-hidden rounded-lg border">
                          <div className="border-b bg-muted/40 px-3 py-2 text-xs font-medium">
                            {t.admin.marketingPreviewEmail}
                          </div>
                          <iframe
                            title="marketing-email-preview"
                            srcDoc={previewData.email_html || ''}
                            className="w-full border-0"
                            style={{ minHeight: '220px' }}
                            sandbox=""
                          />
                        </div>
                      ) : null}

                      {sendSms ? (
                        <div className="overflow-hidden rounded-lg border">
                          <div className="border-b bg-muted/40 px-3 py-2 text-xs font-medium">
                            {t.admin.marketingPreviewSms}
                          </div>
                          <pre className="m-0 whitespace-pre-wrap break-words bg-background px-3 py-3 text-sm leading-6">
                            {previewData.sms_text || '-'}
                          </pre>
                        </div>
                      ) : null}

                      <div className="rounded-lg border border-dashed p-3">
                        <p className="text-xs text-muted-foreground">
                          {t.admin.marketingPlaceholderHint}
                        </p>
                        {previewData.supported_placeholders &&
                        previewData.supported_placeholders.length > 0 ? (
                          <p className="mt-1 break-all text-[11px] text-muted-foreground">
                            {previewData.supported_placeholders.join('  ')}
                          </p>
                        ) : null}
                        <p className="mt-2 text-xs text-muted-foreground">
                          {t.admin.marketingTemplateVariableHint}
                        </p>
                        {previewData.supported_template_variables &&
                        previewData.supported_template_variables.length > 0 ? (
                          <p className="mt-1 break-all text-[11px] text-muted-foreground">
                            {previewData.supported_template_variables.join('  ')}
                          </p>
                        ) : null}
                      </div>
                    </>
                  ) : (
                    <div className="rounded-lg border p-6 text-center text-sm text-muted-foreground">
                      {t.admin.marketingPreviewEmpty}
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </div>

            <Separator />

            <div className="space-y-3">
              <Label>{t.admin.marketingChannels}</Label>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
                  <div className="flex min-w-0 items-start gap-2">
                    <Mail className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{t.announcement.sendEmail}</p>
                      <p className="text-xs text-muted-foreground">{t.admin.marketingEmailHint}</p>
                    </div>
                  </div>
                  <Switch checked={sendEmail} onCheckedChange={setSendEmail} />
                </div>

                <div className="flex items-center justify-between gap-3 rounded-lg border p-3">
                  <div className="flex min-w-0 items-start gap-2">
                    <MessageSquare className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{t.announcement.sendSms}</p>
                      <p className="text-xs text-muted-foreground">{t.admin.marketingSmsHint}</p>
                    </div>
                  </div>
                  <Switch checked={sendSms} onCheckedChange={setSendSms} />
                </div>
              </div>
              <p className="text-xs text-muted-foreground">{t.admin.marketingRespectPreferences}</p>
            </div>
          </CardContent>
        </Card>

        <Card className="h-fit self-start">
          <CardHeader>
            <CardTitle>{t.admin.marketingRecipients}</CardTitle>
            <CardDescription>{t.admin.marketingRecipientsDesc}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-2">
              <Button
                type="button"
                variant={recipientMode === 'all' ? 'default' : 'outline'}
                onClick={() => setRecipientMode('all')}
              >
                {t.admin.marketingTargetAll}
              </Button>
              <Button
                type="button"
                variant={recipientMode === 'selected' ? 'default' : 'outline'}
                onClick={() => {
                  if (!canViewRecipientUsers) return
                  setRecipientMode('selected')
                }}
                disabled={!canViewRecipientUsers}
              >
                {t.admin.marketingTargetSelected}
              </Button>
            </div>
            {!canViewRecipientUsers ? (
              <p className="text-xs text-muted-foreground">{t.message.noPermission}</p>
            ) : null}

            {recipientMode === 'all' ? (
              <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                {t.admin.marketingTargetAllHint}
              </div>
            ) : (
              <div className="space-y-3">
                <Card>
                  <CardContent className="space-y-3 pt-6">
                    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                      <div className="space-y-2">
                        <label className="text-sm font-medium">{t.admin.userFilterSearch}</label>
                        <div className="relative">
                          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                          <Input
                            value={searchInput}
                            onChange={(e) => setSearchInput(e.target.value)}
                            placeholder={t.admin.userFilterSearchPlaceholder}
                            className="pl-9"
                          />
                        </div>
                      </div>

                      <div className="space-y-2">
                        <label className="text-sm font-medium">
                          {cleanFilterLabel(t.admin.userFilterStatus)}
                        </label>
                        <Select
                          value={userStatusFilter}
                          onValueChange={(v) => {
                            setUserStatusFilter(v as TriState)
                            setUserPage(1)
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder={cleanFilterLabel(t.admin.userFilterStatus)} />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">{t.common.all}</SelectItem>
                            <SelectItem value="true">{t.admin.active}</SelectItem>
                            <SelectItem value="false">{t.admin.inactive}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="space-y-2">
                        <label className="text-sm font-medium">
                          {cleanFilterLabel(t.admin.userFilterEmailVerified)}
                        </label>
                        <Select
                          value={userVerifiedFilter}
                          onValueChange={(v) => {
                            setUserVerifiedFilter(v as TriState)
                            setUserPage(1)
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue
                              placeholder={cleanFilterLabel(t.admin.userFilterEmailVerified)}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">{t.common.all}</SelectItem>
                            <SelectItem value="true">{t.admin.verified}</SelectItem>
                            <SelectItem value="false">{t.admin.unverified}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="space-y-2">
                        <label className="text-sm font-medium">
                          {cleanFilterLabel(t.admin.userFilterHasPhone)}
                        </label>
                        <Select
                          value={userHasPhoneFilter}
                          onValueChange={(v) => {
                            setUserHasPhoneFilter(v as TriState)
                            setUserPage(1)
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue
                              placeholder={cleanFilterLabel(t.admin.userFilterHasPhone)}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">{t.common.all}</SelectItem>
                            <SelectItem value="true">{t.admin.withPhone}</SelectItem>
                            <SelectItem value="false">{t.admin.withoutPhone}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="space-y-2">
                        <label className="text-sm font-medium">
                          {cleanFilterLabel(t.admin.userFilterEmailMarketing)}
                        </label>
                        <Select
                          value={userEmailMarketingFilter}
                          onValueChange={(v) => {
                            setUserEmailMarketingFilter(v as TriState)
                            setUserPage(1)
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue
                              placeholder={cleanFilterLabel(t.admin.userFilterEmailMarketing)}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">{t.common.all}</SelectItem>
                            <SelectItem value="true">{t.admin.enabled}</SelectItem>
                            <SelectItem value="false">{t.admin.disabled}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="space-y-2">
                        <label className="text-sm font-medium">
                          {cleanFilterLabel(t.admin.userFilterSmsMarketing)}
                        </label>
                        <Select
                          value={userSmsMarketingFilter}
                          onValueChange={(v) => {
                            setUserSmsMarketingFilter(v as TriState)
                            setUserPage(1)
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue
                              placeholder={cleanFilterLabel(t.admin.userFilterSmsMarketing)}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">{t.common.all}</SelectItem>
                            <SelectItem value="true">{t.admin.enabled}</SelectItem>
                            <SelectItem value="false">{t.admin.disabled}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>

                      <div className="space-y-2">
                        <label className="text-sm font-medium">
                          {cleanFilterLabel(t.admin.userFilterLocale)}
                        </label>
                        <Select
                          value={userLocaleFilter || 'all'}
                          onValueChange={(value) => {
                            setUserLocaleFilter(value === 'all' ? '' : value)
                            setUserPage(1)
                          }}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder={t.common.all} />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="all">{t.common.all}</SelectItem>
                            <SelectItem value="zh">{t.language.zh}</SelectItem>
                            <SelectItem value="en">{t.language.en}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  </CardContent>
                </Card>

                <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                  <div className="flex items-center gap-2">
                    <Users className="h-3.5 w-3.5" />
                    {t.admin.marketingSelectedCount.replace(
                      '{count}',
                      String(selectedUserIds.length)
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={handleToggleCurrentPage}
                      disabled={
                        usersQuery.isLoading ||
                        usersQuery.isFetching ||
                        selectableCurrentPageIds.length === 0
                      }
                    >
                      {allCurrentPageSelected
                        ? t.admin.marketingUnselectPage
                        : t.admin.marketingSelectPage}
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedUserIds([])}
                      disabled={selectedUserIds.length === 0}
                    >
                      {t.admin.marketingClearSelection}
                    </Button>
                  </div>
                </div>

                <div className="rounded-lg border">
                  <div className="max-h-[320px] space-y-1 overflow-y-auto p-2">
                    {usersQuery.isLoading || usersQuery.isFetching ? (
                      <div className="flex items-center justify-center py-8 text-muted-foreground">
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        {t.common.loading}
                      </div>
                    ) : usersQuery.error ? (
                      <div className="py-8 text-center text-sm text-destructive">
                        {t.admin.marketingLoadUsersFailed}
                      </div>
                    ) : users.length === 0 ? (
                      <div className="py-8 text-center text-sm text-muted-foreground">
                        {t.admin.noData}
                      </div>
                    ) : (
                      users.map((user) => {
                        const isActive = user.is_active !== false
                        const checked = selectedSet.has(user.id)
                        const rowExtensions =
                          adminMarketingRecipientActionExtensions[String(user.id)] || []
                        return (
                          <div
                            key={user.id}
                            className={`rounded-md p-2 transition-colors ${
                              isActive ? 'hover:bg-muted/60' : 'opacity-60'
                            }`}
                          >
                            <label
                              className={`flex items-start gap-3 ${
                                isActive ? 'cursor-pointer' : 'cursor-not-allowed'
                              }`}
                            >
                              <Checkbox
                                checked={checked}
                                disabled={!isActive}
                                onCheckedChange={(value) => toggleUser(user.id, value === true)}
                                className="mt-0.5"
                              />
                              <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-2">
                                  <p className="truncate text-sm font-medium">
                                    {user.name || user.email || `#${user.id}`}
                                  </p>
                                  <Badge variant="outline" className="text-[10px]">
                                    #{user.id}
                                  </Badge>
                                  {!isActive ? (
                                    <Badge variant="secondary" className="text-[10px]">
                                      {t.admin.inactive}
                                    </Badge>
                                  ) : null}
                                </div>
                                <p className="truncate text-xs text-muted-foreground">
                                  {user.email || '-'}
                                </p>
                                <p className="truncate text-xs text-muted-foreground">
                                  {user.phone || '-'}
                                </p>
                              </div>
                            </label>
                            {rowExtensions.length > 0 ? (
                              <div className="mt-2 pl-7">
                                <PluginExtensionList extensions={rowExtensions} display="inline" />
                              </div>
                            ) : null}
                          </div>
                        )
                      })
                    )}
                  </div>
                </div>

                {totalUserPages > 1 ? (
                  <div className="flex items-center justify-between gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setUserPage((p) => p - 1)}
                      disabled={userPage <= 1}
                    >
                      {t.admin.prevPage}
                    </Button>
                    <span className="text-xs text-muted-foreground">
                      {t.admin.page
                        .replace('{current}', String(userPage))
                        .replace('{total}', String(totalUserPages))}
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setUserPage((p) => p + 1)}
                      disabled={userPage >= totalUserPages}
                    >
                      {t.admin.nextPage}
                    </Button>
                  </div>
                ) : null}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {activeBatch ? (
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.marketingResult}</CardTitle>
            <CardDescription>{t.admin.marketingResultDesc}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-8">
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingBatchNo}</p>
                <p className="mt-1 break-all text-sm font-semibold">
                  {activeBatch.batch_no || '-'}
                </p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingOperator}</p>
                <p className="mt-1 text-sm font-semibold">{activeBatch.operator_name || '-'}</p>
                <p className="mt-1 text-[11px] text-muted-foreground">
                  {formatDateTime(activeBatch.created_at, locale)}
                </p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingStatus}</p>
                <p className="mt-1 text-sm font-semibold">
                  {getBatchStatusText(activeBatchStatus)}
                </p>
                {activeBatch.failed_reason ? (
                  <p className="mt-1 line-clamp-2 text-[11px] text-destructive">
                    {activeBatch.failed_reason}
                  </p>
                ) : null}
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingProgress}</p>
                <p className="mt-1 text-sm font-semibold">{activeBatchProgress}</p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingTargetedUsers}</p>
                <p className="mt-1 text-xl font-semibold">{activeBatch.targeted_users}</p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingEmailSummary}</p>
                <p className="mt-1 text-sm">
                  {t.admin.marketingSent}: {activeBatch.email_sent} / {t.admin.marketingFailed}:{' '}
                  {activeBatch.email_failed} / {t.admin.marketingSkipped}:{' '}
                  {activeBatch.email_skipped}
                </p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingSmsSummary}</p>
                <p className="mt-1 text-sm">
                  {t.admin.marketingSent}: {activeBatch.sms_sent} / {t.admin.marketingFailed}:{' '}
                  {activeBatch.sms_failed} / {t.admin.marketingSkipped}: {activeBatch.sms_skipped}
                </p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingRequestUsers}</p>
                <p className="mt-1 text-xl font-semibold">{activeBatch.requested_user_count}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>{t.admin.marketingHistory}</CardTitle>
          <CardDescription>{t.admin.marketingHistoryDesc}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {batchesQuery.isLoading || batchesQuery.isFetching ? (
            <div className="flex items-center justify-center py-6 text-sm text-muted-foreground">
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              {t.common.loading}
            </div>
          ) : batches.length === 0 ? (
            <div className="py-6 text-center text-sm text-muted-foreground">{t.admin.noData}</div>
          ) : (
            batches.map((batch) => {
              const rowExtensions = adminMarketingBatchActionExtensions[String(batch.id)] || []
              return (
                <div
                  key={batch.id}
                  className={`cursor-pointer rounded-lg border p-3 transition-colors ${
                    lastBatchId === batch.id ? 'border-primary bg-primary/5' : 'hover:bg-muted/40'
                  }`}
                  onClick={() => setLastBatchId(batch.id)}
                >
                  <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                    <div className="min-w-0">
                      <p className="break-all text-sm font-semibold">{batch.batch_no}</p>
                      <p className="truncate text-xs text-muted-foreground">{batch.title}</p>
                    </div>
                    <div className="text-right">
                      <p className="text-xs text-muted-foreground">
                        {t.admin.marketingOperator}: {batch.operator_name || '-'}
                      </p>
                      <p className="text-xs font-medium">
                        {t.admin.marketingStatus}: {getBatchStatusText(batch.status)}
                      </p>
                    </div>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                    <span>
                      {t.admin.createdAt}: {formatDateTime(batch.created_at, locale)}
                    </span>
                    <span>
                      {t.admin.marketingProgress}: {batch.processed_tasks}/{batch.total_tasks}
                    </span>
                    <span>
                      {t.admin.marketingTargetedUsers}: {batch.targeted_users}
                    </span>
                    <span>
                      {t.admin.marketingEmailSummary}: {batch.email_sent}/{batch.email_failed}/
                      {batch.email_skipped}
                    </span>
                    <span>
                      {t.admin.marketingSmsSummary}: {batch.sms_sent}/{batch.sms_failed}/
                      {batch.sms_skipped}
                    </span>
                  </div>
                  {rowExtensions.length > 0 ? (
                    <div className="mt-3" onClick={(event) => event.stopPropagation()}>
                      <PluginExtensionList extensions={rowExtensions} display="inline" />
                    </div>
                  ) : null}
                </div>
              )
            })
          )}

          {totalBatchPages > 1 ? (
            <div className="flex items-center justify-between gap-2 pt-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setBatchPage((p) => p - 1)}
                disabled={batchPage <= 1}
              >
                {t.admin.prevPage}
              </Button>
              <span className="text-xs text-muted-foreground">
                {t.admin.page
                  .replace('{current}', String(batchPage))
                  .replace('{total}', String(totalBatchPages))}
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setBatchPage((p) => p + 1)}
                disabled={batchPage >= totalBatchPages}
              >
                {t.admin.nextPage}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}
