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
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import { Loader2, Mail, MessageSquare, Search, Send, Users } from 'lucide-react'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { usePermission } from '@/hooks/use-permission'

type RecipientMode = 'all' | 'selected'

interface AdminUserItem {
  id: number
  name?: string
  email?: string
  phone?: string | null
  role?: string
  is_active?: boolean
}

function formatDateTime(dateString?: string, locale?: string) {
  if (!dateString) return '-'
  const date = new Date(dateString)
  if (Number.isNaN(date.getTime())) return dateString
  return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })
}

export default function AdminMarketingPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()
  const { hasPermission } = usePermission()
  const [permissionReady, setPermissionReady] = useState(false)
  const canViewMarketing = permissionReady && hasPermission('marketing.view')
  const canSendMarketing = permissionReady && hasPermission('marketing.send')
  usePageTitle(t.pageTitle.adminMarketing)

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
  const [userPage, setUserPage] = useState(1)
  const [batchPage, setBatchPage] = useState(1)
  const [lastBatchId, setLastBatchId] = useState<number | null>(null)
  const [lastResult, setLastResult] = useState<SendAdminMarketingResult | null>(null)

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setPreviewTitle(title)
      setPreviewContent(content)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [title, content])

  const usersQuery = useQuery({
    queryKey: ['marketingUsers', userPage, search],
    queryFn: () => getMarketingUsers({ page: userPage, limit: 20, search: search || undefined }),
    enabled: canViewMarketing && recipientMode === 'selected',
  })

  const users: AdminUserItem[] = usersQuery.data?.data?.items || []
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
    queryFn: () => previewAdminMarketing({
      title: previewTitle.trim(),
      content: previewContent,
      user_id: previewUserId,
    }),
    enabled: canSendMarketing
      && contentTab === 'preview'
      && (previewTitle.trim().length > 0 || previewContent.trim().length > 0),
  })

  const previewData = previewQuery.data?.data as PreviewAdminMarketingResult | undefined

  const batchesQuery = useQuery({
    queryKey: ['marketingBatches', batchPage],
    queryFn: () => getMarketingBatches({ page: batchPage, limit: 8 }),
    enabled: canViewMarketing,
  })

  const batches: MarketingBatchItem[] = batchesQuery.data?.data?.items || []
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
  const allCurrentPageSelected = selectableCurrentPageIds.length > 0
    && selectableCurrentPageIds.every((id) => selectedSet.has(id))

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
    onError: (error: Error) => {
      toast.error(`${t.admin.marketingSendFailed}: ${error.message}`)
    },
  })

  const activeBatch = (batchDetailQuery.data?.data as MarketingBatchItem | undefined) || (lastResult as unknown as MarketingBatchItem | null)
  const activeBatchStatus = activeBatch?.status
  const activeBatchProgress = activeBatch && typeof activeBatch.total_tasks === 'number' && activeBatch.total_tasks > 0
    ? `${activeBatch.processed_tasks}/${activeBatch.total_tasks}`
    : '-'

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
          <h1 className="text-2xl md:text-3xl font-bold">{t.admin.marketingManagement}</h1>
          <p className="text-sm text-muted-foreground mt-1">{t.admin.marketingDescription}</p>
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
          <h1 className="text-2xl md:text-3xl font-bold">{t.admin.marketingManagement}</h1>
          <p className="text-sm text-muted-foreground mt-1">{t.admin.marketingDescription}</p>
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
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-2xl md:text-3xl font-bold">{t.admin.marketingManagement}</h1>
          <p className="text-sm text-muted-foreground mt-1">{t.admin.marketingDescription}</p>
          {!canSendMarketing ? (
            <p className="text-xs text-amber-600 mt-2">{t.admin.marketingNoSendPermission}</p>
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
              <Tabs value={contentTab} onValueChange={(value) => setContentTab(value as 'edit' | 'preview')}>
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
                    <div className="rounded-lg border p-6 text-sm text-muted-foreground flex items-center justify-center">
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t.admin.marketingPreviewLoading}
                    </div>
                  ) : previewData ? (
                    <>
                      {sendEmail ? (
                        <div className="rounded-lg border overflow-hidden">
                          <div className="px-3 py-2 border-b bg-muted/40 text-xs font-medium">
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
                        <div className="rounded-lg border overflow-hidden">
                          <div className="px-3 py-2 border-b bg-muted/40 text-xs font-medium">
                            {t.admin.marketingPreviewSms}
                          </div>
                          <pre className="m-0 whitespace-pre-wrap break-words px-3 py-3 text-sm leading-6 bg-background">
                            {previewData.sms_text || '-'}
                          </pre>
                        </div>
                      ) : null}

                      <div className="rounded-lg border border-dashed p-3">
                        <p className="text-xs text-muted-foreground">{t.admin.marketingPlaceholderHint}</p>
                        {previewData.supported_placeholders && previewData.supported_placeholders.length > 0 ? (
                          <p className="mt-1 text-[11px] break-all text-muted-foreground">
                            {previewData.supported_placeholders.join('  ')}
                          </p>
                        ) : null}
                        <p className="mt-2 text-xs text-muted-foreground">{t.admin.marketingTemplateVariableHint}</p>
                        {previewData.supported_template_variables && previewData.supported_template_variables.length > 0 ? (
                          <p className="mt-1 text-[11px] break-all text-muted-foreground">
                            {previewData.supported_template_variables.join('  ')}
                          </p>
                        ) : null}
                      </div>
                    </>
                  ) : (
                    <div className="rounded-lg border p-6 text-sm text-muted-foreground text-center">
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
                <div className="rounded-lg border p-3 flex items-center justify-between gap-3">
                  <div className="flex items-start gap-2 min-w-0">
                    <Mail className="h-4 w-4 mt-0.5 shrink-0 text-muted-foreground" />
                    <div className="min-w-0">
                      <p className="text-sm font-medium">{t.announcement.sendEmail}</p>
                      <p className="text-xs text-muted-foreground">{t.admin.marketingEmailHint}</p>
                    </div>
                  </div>
                  <Switch checked={sendEmail} onCheckedChange={setSendEmail} />
                </div>

                <div className="rounded-lg border p-3 flex items-center justify-between gap-3">
                  <div className="flex items-start gap-2 min-w-0">
                    <MessageSquare className="h-4 w-4 mt-0.5 shrink-0 text-muted-foreground" />
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
                onClick={() => setRecipientMode('selected')}
              >
                {t.admin.marketingTargetSelected}
              </Button>
            </div>

            {recipientMode === 'all' ? (
              <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                {t.admin.marketingTargetAllHint}
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      value={searchInput}
                      onChange={(e) => setSearchInput(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          setUserPage(1)
                          setSearch(searchInput.trim())
                        }
                      }}
                      placeholder={t.admin.marketingUserSearchPlaceholder}
                      className="pl-9"
                    />
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setUserPage(1)
                      setSearch(searchInput.trim())
                    }}
                  >
                    {t.admin.search}
                  </Button>
                </div>

                <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
                  <div className="flex items-center gap-2">
                    <Users className="h-3.5 w-3.5" />
                    {t.admin.marketingSelectedCount.replace('{count}', String(selectedUserIds.length))}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={handleToggleCurrentPage}
                      disabled={usersQuery.isLoading || usersQuery.isFetching || selectableCurrentPageIds.length === 0}
                    >
                      {allCurrentPageSelected ? t.admin.marketingUnselectPage : t.admin.marketingSelectPage}
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
                  <div className="max-h-[320px] overflow-y-auto p-2 space-y-1">
                      {usersQuery.isLoading || usersQuery.isFetching ? (
                        <div className="flex items-center justify-center py-8 text-muted-foreground">
                          <Loader2 className="h-4 w-4 animate-spin mr-2" />
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
                          return (
                            <label
                              key={user.id}
                              className={`flex items-start gap-3 rounded-md p-2 transition-colors ${
                                isActive ? 'hover:bg-muted/60 cursor-pointer' : 'opacity-60 cursor-not-allowed'
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
                                  <p className="text-sm font-medium truncate">
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
                                <p className="text-xs text-muted-foreground truncate">{user.email || '-'}</p>
                                <p className="text-xs text-muted-foreground truncate">{user.phone || '-'}</p>
                              </div>
                            </label>
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
                <p className="mt-1 text-sm font-semibold break-all">{activeBatch.batch_no || '-'}</p>
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
                <p className="mt-1 text-sm font-semibold">{getBatchStatusText(activeBatchStatus)}</p>
                {activeBatch.failed_reason ? (
                  <p className="mt-1 line-clamp-2 text-[11px] text-destructive">{activeBatch.failed_reason}</p>
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
                  {t.admin.marketingSent}: {activeBatch.email_sent} / {t.admin.marketingFailed}: {activeBatch.email_failed} / {t.admin.marketingSkipped}: {activeBatch.email_skipped}
                </p>
              </div>
              <div className="rounded-lg border p-3">
                <p className="text-xs text-muted-foreground">{t.admin.marketingSmsSummary}</p>
                <p className="mt-1 text-sm">
                  {t.admin.marketingSent}: {activeBatch.sms_sent} / {t.admin.marketingFailed}: {activeBatch.sms_failed} / {t.admin.marketingSkipped}: {activeBatch.sms_skipped}
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
            batches.map((batch) => (
              <div
                key={batch.id}
                className={`rounded-lg border p-3 transition-colors cursor-pointer ${
                  lastBatchId === batch.id ? 'border-primary bg-primary/5' : 'hover:bg-muted/40'
                }`}
                onClick={() => setLastBatchId(batch.id)}
              >
                <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <p className="text-sm font-semibold break-all">{batch.batch_no}</p>
                    <p className="text-xs text-muted-foreground truncate">{batch.title}</p>
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
                  <span>{t.admin.createdAt}: {formatDateTime(batch.created_at, locale)}</span>
                  <span>{t.admin.marketingProgress}: {batch.processed_tasks}/{batch.total_tasks}</span>
                  <span>{t.admin.marketingTargetedUsers}: {batch.targeted_users}</span>
                  <span>{t.admin.marketingEmailSummary}: {batch.email_sent}/{batch.email_failed}/{batch.email_skipped}</span>
                  <span>{t.admin.marketingSmsSummary}: {batch.sms_sent}/{batch.sms_failed}/{batch.sms_skipped}</span>
                </div>
              </div>
            ))
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
