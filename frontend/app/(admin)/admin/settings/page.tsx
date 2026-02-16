'use client'

import { useState, useEffect, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getSettings, updateSettings, testSMTP, getEmailTemplates, getEmailTemplate, updateEmailTemplate, getLandingPage, updateLandingPage, resetLandingPage } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/hooks/use-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { Settings, Database, Mail, Shield, Zap, Package, Save, TestTube, FileUp, Globe, Cloud, FileText, MessageSquare, Sun, Moon, Monitor, Image, Mic, Palette, Plus, Trash2, FileCode, ShieldCheck, Layout, RotateCcw, BarChart3 } from 'lucide-react'
import { useForm } from 'react-hook-form'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

// 单独的页面规则编辑卡片组件，使用本地state避免每次输入都重渲染整个设置页面
interface PageRule {
  name: string; pattern: string; match_type: string; css: string; js: string; enabled: boolean
}

function PageRuleItem({
  rule,
  index,
  onChange,
  onDelete,
  t,
}: {
  rule: PageRule
  index: number
  onChange: (index: number, updated: PageRule) => void
  onDelete: (index: number) => void
  t: any
}) {
  // 所有字段使用本地state，只在blur时同步到父组件，避免每次按键都触发2100行父组件重渲染
  const [local, setLocal] = useState(rule)

  // 父组件重置时（如加载远程数据）同步到本地
  useEffect(() => {
    setLocal(rule)
  }, [rule])

  const handleBlur = () => {
    onChange(index, local)
  }

  return (
    <Card className={`border ${local.enabled ? 'border-primary/30' : 'border-muted opacity-60'}`}>
      <CardContent className="pt-4 space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex-1 grid grid-cols-1 md:grid-cols-3 gap-2">
            <div>
              <Label className="text-xs">{t.admin.ruleName}</Label>
              <Input
                value={local.name}
                onChange={(e) => setLocal(prev => ({ ...prev, name: e.target.value }))}
                onBlur={handleBlur}
                placeholder={t.admin.ruleName}
                className="mt-1 h-8 text-sm"
              />
            </div>
            <div>
              <Label className="text-xs">{t.admin.matchPattern}</Label>
              <Input
                value={local.pattern}
                onChange={(e) => setLocal(prev => ({ ...prev, pattern: e.target.value }))}
                onBlur={handleBlur}
                placeholder={local.match_type === 'regex' ? '^/products/[^/]+$' : '/products'}
                className="mt-1 h-8 text-sm font-mono"
              />
            </div>
            <div>
              <Label className="text-xs">{t.admin.matchType}</Label>
              <Select
                value={local.match_type}
                onValueChange={(v) => {
                  const updated = { ...local, match_type: v }
                  setLocal(updated)
                  onChange(index, updated)
                }}
              >
                <SelectTrigger className="mt-1 h-8 text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="exact">{t.admin.exactMatch}</SelectItem>
                  <SelectItem value="regex">{t.admin.regexMatch}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex items-center gap-1 pt-4">
            <Switch
              checked={local.enabled}
              onCheckedChange={(v) => {
                const updated = { ...local, enabled: v }
                setLocal(updated)
                onChange(index, updated)
              }}
            />
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 w-8 p-0 text-destructive hover:text-destructive"
              onClick={() => onDelete(index)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <Label className="text-xs">CSS</Label>
            <textarea
              value={local.css}
              onChange={(e) => setLocal(prev => ({ ...prev, css: e.target.value }))}
              onBlur={handleBlur}
              placeholder={t.admin.cssPlaceholder}
              rows={4}
              className="mt-1 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
            />
          </div>
          <div>
            <Label className="text-xs">JavaScript</Label>
            <textarea
              value={local.js}
              onChange={(e) => setLocal(prev => ({ ...prev, js: e.target.value }))}
              onBlur={handleBlur}
              placeholder={t.admin.jsPlaceholder}
              rows={4}
              className="mt-1 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-xs"
            />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// 邮件模板编辑器组件，使用本地state避免每次按键重渲染整个设置页面
function TemplateEditor({
  content,
  onChange,
  onSave,
  isSaving,
  isPreview,
  t,
}: {
  content: string
  onChange: (content: string) => void
  onSave: (content: string) => void
  isSaving: boolean
  isPreview: boolean
  t: any
}) {
  const [local, setLocal] = useState(content)

  useEffect(() => {
    setLocal(content)
  }, [content])

  if (isPreview) {
    return (
      <>
        <div className="border rounded-md overflow-hidden bg-white">
          <iframe
            srcDoc={local}
            className="w-full border-0"
            style={{ minHeight: '500px' }}
            title="Template Preview"
            sandbox=""
          />
        </div>
        <div className="flex items-center gap-2">
          <Button
            onClick={() => onSave(local)}
            disabled={isSaving}
          >
            <Save className="mr-2 h-4 w-4" />
            {isSaving ? t.admin.saving : t.admin.saveTemplate}
          </Button>
          <p className="text-xs text-muted-foreground">
            {t.admin.templateTip}
          </p>
        </div>
      </>
    )
  }

  return (
    <>
      <textarea
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        onBlur={() => onChange(local)}
        className="w-full min-h-[500px] p-3 font-mono text-sm border rounded-md bg-muted/50 focus:outline-none focus:ring-2 focus:ring-ring resize-y"
        spellCheck={false}
      />
      <div className="flex items-center gap-2">
        <Button
          onClick={() => onSave(local)}
          disabled={isSaving}
        >
          <Save className="mr-2 h-4 w-4" />
          {isSaving ? t.admin.saving : t.admin.saveTemplate}
        </Button>
        <p className="text-xs text-muted-foreground">
          {t.admin.templateTip}
        </p>
      </div>
    </>
  )
}

export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState('general')
  const [defaultTheme, setDefaultTheme] = useState('system')
  const [primaryColor, setPrimaryColor] = useState('')
  const [pageRules, setPageRules] = useState<PageRule[]>([])
  const [emailNotifications, setEmailNotifications] = useState<Record<string, boolean>>({})
  const [captchaProvider, setCaptchaProvider] = useState('none')
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminSettings)

  const { data: settings, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  })

  const updateMutation = useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      toast.success(t.admin.settingsSaved)
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['publicConfig'] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.saveFailed)
    },
  })

  const testSMTPMutation = useMutation({
    mutationFn: testSMTP,
    onSuccess: () => {
      toast.success(t.admin.testEmailSent)
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.testFailed)
    },
  })

  // 邮件模板编辑
  const [selectedTemplate, setSelectedTemplate] = useState('')
  const [templateContent, setTemplateContent] = useState('')
  const [templatePreview, setTemplatePreview] = useState(false)

  const { data: emailTemplatesData } = useQuery({
    queryKey: ['emailTemplates'],
    queryFn: getEmailTemplates,
    enabled: activeTab === 'smtp',
  })

  const { data: templateData, isFetching: isTemplateFetching } = useQuery({
    queryKey: ['emailTemplate', selectedTemplate],
    queryFn: () => getEmailTemplate(selectedTemplate),
    enabled: !!selectedTemplate,
  })

  useEffect(() => {
    if (templateData?.data?.content) {
      setTemplateContent(templateData.data.content)
    }
  }, [templateData])

  const saveTemplateMutation = useMutation({
    mutationFn: ({ filename, content }: { filename: string; content: string }) =>
      updateEmailTemplate(filename, content),
    onSuccess: () => {
      toast.success(t.admin.templateSaved)
      queryClient.invalidateQueries({ queryKey: ['emailTemplate', selectedTemplate] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.saveFailed)
    },
  })

  // 落地页编辑
  const [landingHtml, setLandingHtml] = useState('')

  const { data: landingPageData } = useQuery({
    queryKey: ['landingPage'],
    queryFn: getLandingPage,
    enabled: activeTab === 'landingpage',
  })

  useEffect(() => {
    if (landingPageData?.data?.html_content) {
      setLandingHtml(landingPageData.data.html_content)
    }
  }, [landingPageData])

  const saveLandingPageMutation = useMutation({
    mutationFn: (html: string) => updateLandingPage(html),
    onSuccess: () => {
      toast.success(t.admin.landingPageSaved)
      queryClient.invalidateQueries({ queryKey: ['landingPage'] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.landingPageSaveFailed)
    },
  })

  const resetLandingPageMutation = useMutation({
    mutationFn: () => resetLandingPage(),
    onSuccess: (res: any) => {
      toast.success(t.admin.landingPageResetSuccess)
      if (res?.data?.html_content) {
        setLandingHtml(res.data.html_content)
      }
      queryClient.invalidateQueries({ queryKey: ['landingPage'] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.landingPageResetFailed)
    },
  })

  const templateEventLabels: Record<string, string> = {
    welcome: t.admin.templateEventWelcome,
    email_verification: t.admin.templateEventEmailVerification,
    order_created: t.admin.templateEventOrderCreated,
    order_paid: t.admin.templateEventOrderPaid,
    order_shipped: t.admin.templateEventOrderShipped,
    order_completed: t.admin.templateEventOrderCompleted,
    order_cancelled: t.admin.templateEventOrderCancelled,
    order_resubmit: t.admin.templateEventOrderResubmit,
    ticket_created: t.admin.templateEventTicketCreated,
    ticket_reply: t.admin.templateEventTicketReply,
    ticket_resolved: t.admin.templateEventTicketResolved,
  }

  const settingsData = settings?.data

  // 当设置数据加载后，初始化默认主题和个性化配置
  useEffect(() => {
    if (settingsData?.app?.default_theme) {
      setDefaultTheme(settingsData.app.default_theme)
    }
    if (settingsData?.customization?.primary_color !== undefined) {
      setPrimaryColor(settingsData.customization.primary_color)
    }
    if (settingsData?.customization?.page_rules) {
      setPageRules(settingsData.customization.page_rules)
    }
    if (settingsData?.email_notifications) {
      setEmailNotifications(settingsData.email_notifications)
    }
    if (settingsData?.security?.captcha?.provider) {
      setCaptchaProvider(settingsData.security.captcha.provider)
    }
  }, [settingsData])

  // 处理表单提交
  const handleSubmit = (section: string, data: any) => {
    const payload: any = {}
    payload[section] = data
    updateMutation.mutate(payload)
  }

  const handlePageRuleUpdate = useCallback((index: number, updated: PageRule) => {
    setPageRules(prev => {
      const next = [...prev]
      next[index] = updated
      return next
    })
  }, [])

  const handlePageRuleDelete = useCallback((index: number) => {
    setPageRules(prev => prev.filter((_, i) => i !== index))
  }, [])

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-3xl font-bold">{t.admin.systemSettings}</h1>
        <div className="text-center py-8">{t.common.loading}</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.admin.systemSettings}</h1>
        <Badge variant="secondary">{t.admin.superAdminBadge}</Badge>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="inline-flex overflow-x-auto h-auto flex-wrap gap-1 p-1">
          <TabsTrigger value="general" className="gap-1.5 px-3">
            <Settings className="h-4 w-4 shrink-0" />
            {t.admin.tabGeneral}
          </TabsTrigger>
          <TabsTrigger value="smtp" className="gap-1.5 px-3">
            <Mail className="h-4 w-4 shrink-0" />
            {t.admin.tabEmail}
          </TabsTrigger>
          <TabsTrigger value="security" className="gap-1.5 px-3">
            <Shield className="h-4 w-4 shrink-0" />
            {t.admin.tabSecurity}
          </TabsTrigger>
          <TabsTrigger value="ratelimit" className="gap-1.5 px-3">
            <Zap className="h-4 w-4 shrink-0" />
            {t.admin.tabRateLimit}
          </TabsTrigger>
          <TabsTrigger value="order" className="gap-1.5 px-3">
            <Package className="h-4 w-4 shrink-0" />
            {t.admin.tabOrder}
          </TabsTrigger>
          <TabsTrigger value="ticket" className="gap-1.5 px-3">
            <MessageSquare className="h-4 w-4 shrink-0" />
            {t.admin.tabTicket}
          </TabsTrigger>
          <TabsTrigger value="serial" className="gap-1.5 px-3">
            <ShieldCheck className="h-4 w-4 shrink-0" />
            {t.admin.tabSerial}
          </TabsTrigger>
          <TabsTrigger value="analytics" className="gap-1.5 px-3">
            <BarChart3 className="h-4 w-4 shrink-0" />
            {t.admin.tabAnalytics}
          </TabsTrigger>
          <TabsTrigger value="upload" className="gap-1.5 px-3">
            <FileUp className="h-4 w-4 shrink-0" />
            {t.admin.tabUpload}
          </TabsTrigger>
          <TabsTrigger value="personalization" className="gap-1.5 px-3">
            <Palette className="h-4 w-4 shrink-0" />
            {t.admin.tabPersonalization}
          </TabsTrigger>
          <TabsTrigger value="landingpage" className="gap-1.5 px-3">
            <Layout className="h-4 w-4 shrink-0" />
            {t.admin.landingPage}
          </TabsTrigger>
          <TabsTrigger value="advanced" className="gap-1.5 px-3">
            <Database className="h-4 w-4 shrink-0" />
            {t.admin.tabAdvanced}
          </TabsTrigger>
        </TabsList>

        {/* 常规设置 */}
        <TabsContent value="general">
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.appSettings}</CardTitle>
              <CardDescription>{t.admin.appSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  handleSubmit('app', {
                    name: formData.get('name'),
                    url: formData.get('url'),
                    debug: formData.get('debug') === 'on',
                    default_theme: defaultTheme,
                  })
                }}
                className="space-y-4"
              >
                <div>
                  <Label htmlFor="app_name">{t.admin.appName}</Label>
                  <Input
                    id="app_name"
                    name="name"
                    defaultValue={settingsData?.app?.name || ''}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="app_url">{t.admin.appUrl}</Label>
                  <Input
                    id="app_url"
                    name="url"
                    defaultValue={settingsData?.app?.url || ''}
                    placeholder="http://localhost:3000"
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.appUrlHint}
                  </p>
                </div>

                <div>
                  <Label>{t.admin.defaultTheme}</Label>
                  <Select value={defaultTheme} onValueChange={setDefaultTheme}>
                    <SelectTrigger className="mt-1.5">
                      <SelectValue placeholder={t.admin.selectDefaultTheme} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="light">
                        <div className="flex items-center gap-2">
                          <Sun className="h-4 w-4" />
                          {t.admin.lightMode}
                        </div>
                      </SelectItem>
                      <SelectItem value="dark">
                        <div className="flex items-center gap-2">
                          <Moon className="h-4 w-4" />
                          {t.admin.darkMode}
                        </div>
                      </SelectItem>
                      <SelectItem value="system">
                        <div className="flex items-center gap-2">
                          <Monitor className="h-4 w-4" />
                          {t.admin.followSystem}
                        </div>
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.defaultThemeHint}
                  </p>
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="debug">{t.admin.debugMode}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.debugModeHint}
                    </p>
                  </div>
                  <Switch
                    id="debug"
                    name="debug"
                    defaultChecked={settingsData?.app?.debug}
                  />
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {updateMutation.isPending ? t.admin.saving : t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* SMTP设置 */}
        <TabsContent value="smtp">
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.smtpSettings}</CardTitle>
              <CardDescription>{t.admin.smtpSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  const password = formData.get('password') as string
                  handleSubmit('smtp', {
                    enabled: formData.get('enabled') === 'on',
                    host: formData.get('host'),
                    port: parseInt(formData.get('port') as string),
                    user: formData.get('user'),
                    ...(password && { password }), // 只有填写了才更新
                    from_email: formData.get('from_email'),
                    from_name: formData.get('from_name'),
                  })
                }}
                className="space-y-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="smtp_enabled">{t.admin.enableSmtp}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableSmtpHint}
                    </p>
                  </div>
                  <Switch
                    id="smtp_enabled"
                    name="enabled"
                    defaultChecked={settingsData?.smtp?.enabled}
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <Label htmlFor="smtp_host">{t.admin.smtpServer}</Label>
                    <Input
                      id="smtp_host"
                      name="host"
                      defaultValue={settingsData?.smtp?.host || ''}
                      placeholder="smtp.gmail.com"
                      className="mt-1.5"
                    />
                  </div>
                  <div>
                    <Label htmlFor="smtp_port">{t.admin.port}</Label>
                    <Input
                      id="smtp_port"
                      name="port"
                      type="number"
                      defaultValue={settingsData?.smtp?.port || 587}
                      className="mt-1.5"
                    />
                  </div>
                </div>

                <div>
                  <Label htmlFor="smtp_user">{t.admin.username}</Label>
                  <Input
                    id="smtp_user"
                    name="user"
                    defaultValue={settingsData?.smtp?.user || ''}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="smtp_password">{t.admin.smtpPassword}</Label>
                  <Input
                    id="smtp_password"
                    name="password"
                    type="password"
                    placeholder={t.admin.passwordPlaceholder}
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.passwordSecurityHint}
                  </p>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <Label htmlFor="from_email">{t.admin.senderEmail}</Label>
                    <Input
                      id="from_email"
                      name="from_email"
                      type="email"
                      defaultValue={settingsData?.smtp?.from_email || ''}
                      placeholder="noreply@example.com"
                      className="mt-1.5"
                    />
                  </div>
                  <div>
                    <Label htmlFor="from_name">{t.admin.senderName}</Label>
                    <Input
                      id="from_name"
                      name="from_name"
                      defaultValue={settingsData?.smtp?.from_name || ''}
                      placeholder="AuraLogic"
                      className="mt-1.5"
                    />
                  </div>
                </div>

                <div className="flex gap-2">
                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {updateMutation.isPending ? t.admin.saving : t.admin.saveSettings}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      const toEmail = prompt(t.admin.enterTestEmail)
                      if (!toEmail) return
                      const formData = new FormData(document.querySelector('form') as HTMLFormElement)
                      testSMTPMutation.mutate({
                        host: formData.get('host') as string,
                        port: parseInt(formData.get('port') as string),
                        user: formData.get('user') as string,
                        password: formData.get('password') as string,
                        to_email: toEmail,
                      })
                    }}
                    disabled={testSMTPMutation.isPending}
                  >
                    <TestTube className="mr-2 h-4 w-4" />
                    {testSMTPMutation.isPending ? t.admin.testing : t.admin.testConnection}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>

          {/* 邮件通知开关 */}
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>{t.admin.emailNotificationToggles}</CardTitle>
              <CardDescription>{t.admin.emailNotificationTogglesDesc}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {/* 用户相关 */}
              <div>
                <h4 className="text-sm font-medium mb-3">{t.admin.userSection}</h4>
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.registrationWelcomeEmail}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.registrationWelcomeEmailDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.user_register || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, user_register: v }))}
                    />
                  </div>
                </div>
              </div>

              {/* 订单相关 */}
              <div>
                <h4 className="text-sm font-medium mb-3">{t.admin.orderSection}</h4>
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.orderCreatedNotify}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.orderCreatedNotifyDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.order_created || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, order_created: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.paymentConfirmed}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.paymentConfirmedDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.order_paid || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, order_paid: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.orderShipped}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.orderShippedDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.order_shipped || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, order_shipped: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.orderCompleted}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.orderCompletedDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.order_completed || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, order_completed: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.orderCancelled}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.orderCancelledDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.order_cancelled || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, order_cancelled: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.resubmitRequired}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.resubmitRequiredDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.order_resubmit || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, order_resubmit: v }))}
                    />
                  </div>
                </div>
              </div>

              {/* 工单相关 */}
              <div>
                <h4 className="text-sm font-medium mb-3">{t.admin.ticketSection}</h4>
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.ticketCreatedNotify}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.ticketCreatedNotifyDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.ticket_created || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, ticket_created: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.adminReply}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.adminReplyDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.ticket_admin_reply || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, ticket_admin_reply: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.userReply}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.userReplyDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.ticket_user_reply || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, ticket_user_reply: v }))}
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <Label>{t.admin.ticketResolved}</Label>
                      <p className="text-xs text-muted-foreground mt-0.5">{t.admin.ticketResolvedDesc}</p>
                    </div>
                    <Switch
                      checked={emailNotifications.ticket_resolved || false}
                      onCheckedChange={(v) => setEmailNotifications(prev => ({ ...prev, ticket_resolved: v }))}
                    />
                  </div>
                </div>
              </div>

              <Button
                onClick={() => handleSubmit('email_notifications', emailNotifications)}
                disabled={updateMutation.isPending}
              >
                <Save className="mr-2 h-4 w-4" />
                {updateMutation.isPending
                  ? t.admin.saving
                  : t.admin.saveNotificationSettings}
              </Button>
            </CardContent>
          </Card>

          {/* 邮件模板编辑 */}
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>{t.admin.emailTemplateEditor}</CardTitle>
              <CardDescription>{t.admin.emailTemplateEditorDesc}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label>{t.admin.selectTemplate}</Label>
                  <Select value={selectedTemplate} onValueChange={(v) => { setSelectedTemplate(v); setTemplatePreview(false) }}>
                    <SelectTrigger className="mt-1.5">
                      <SelectValue placeholder={t.admin.chooseTemplate} />
                    </SelectTrigger>
                    <SelectContent>
                      {(emailTemplatesData?.data || []).map((tmpl: any) => (
                        <SelectItem key={tmpl.filename} value={tmpl.filename}>
                          {templateEventLabels[tmpl.event] || tmpl.event}
                          {tmpl.locale ? ` (${tmpl.locale === 'zh' ? t.admin.chinese : 'English'})` : ''}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              {selectedTemplate && (
                <>
                  <div className="flex items-center gap-2">
                    <Button
                      variant={templatePreview ? 'outline' : 'default'}
                      size="sm"
                      onClick={() => setTemplatePreview(false)}
                    >
                      <FileCode className="mr-1.5 h-4 w-4" />
                      {t.admin.code}
                    </Button>
                    <Button
                      variant={templatePreview ? 'default' : 'outline'}
                      size="sm"
                      onClick={() => setTemplatePreview(true)}
                    >
                      <Globe className="mr-1.5 h-4 w-4" />
                      {t.admin.preview}
                    </Button>
                  </div>

                  {isTemplateFetching ? (
                    <div className="text-center py-8 text-muted-foreground">
                      {t.common.loading}
                    </div>
                  ) : (
                    <TemplateEditor
                      content={templateContent}
                      onChange={setTemplateContent}
                      onSave={(content) => saveTemplateMutation.mutate({ filename: selectedTemplate, content })}
                      isSaving={saveTemplateMutation.isPending}
                      isPreview={templatePreview}
                      t={t}
                    />
                  )}
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* 安全设置 */}
        <TabsContent value="security">
          <div className="space-y-4">
            {/* 登录设置 */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.loginSettings}</CardTitle>
                <CardDescription>{t.admin.loginSettingsDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('security', {
                      login_submitted: true,
                      login: {
                        allow_password_login: formData.get('allow_password_login') === 'on',
                        allow_registration: formData.get('allow_registration') === 'on',
                        require_email_verification: formData.get('require_email_verification') === 'on',
                      },
                    })
                  }}
                  className="space-y-4"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <Label htmlFor="allow_password_login">{t.admin.allowPasswordLogin}</Label>
                      <p className="text-xs text-muted-foreground mt-1">
                        {t.admin.allowPasswordLoginHint}
                      </p>
                    </div>
                    <Switch
                      id="allow_password_login"
                      name="allow_password_login"
                      defaultChecked={settingsData?.security?.login?.allow_password_login}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <Label htmlFor="allow_registration">{t.admin.allowRegistration}</Label>
                      <p className="text-xs text-muted-foreground mt-1">
                        {t.admin.allowRegistrationHint}
                      </p>
                    </div>
                    <Switch
                      id="allow_registration"
                      name="allow_registration"
                      defaultChecked={settingsData?.security?.login?.allow_registration}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div>
                      <Label htmlFor="require_email_verification">{t.admin.requireEmailVerification}</Label>
                      <p className="text-xs text-muted-foreground mt-1">
                        {t.admin.requireEmailVerificationHint}
                      </p>
                    </div>
                    <Switch
                      id="require_email_verification"
                      name="require_email_verification"
                      defaultChecked={settingsData?.security?.login?.require_email_verification}
                    />
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            {/* 密码策略 */}
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.passwordPolicy}</CardTitle>
                <CardDescription>{t.admin.passwordPolicyDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('security', {
                      password_policy: {
                        min_length: parseInt(formData.get('min_length') as string),
                        require_uppercase: formData.get('require_uppercase') === 'on',
                        require_lowercase: formData.get('require_lowercase') === 'on',
                        require_number: formData.get('require_number') === 'on',
                        require_special: formData.get('require_special') === 'on',
                      },
                    })
                  }}
                  className="space-y-4"
                >
                  <div>
                    <Label htmlFor="min_length">{t.admin.minLength}</Label>
                    <Input
                      id="min_length"
                      name="min_length"
                      type="number"
                      min="6"
                      max="32"
                      defaultValue={settingsData?.security?.password_policy?.min_length || 8}
                      className="mt-1.5"
                    />
                  </div>

                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <Label htmlFor="require_uppercase">{t.admin.requireUppercase}</Label>
                      <Switch
                        id="require_uppercase"
                        name="require_uppercase"
                        defaultChecked={settingsData?.security?.password_policy?.require_uppercase}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <Label htmlFor="require_lowercase">{t.admin.requireLowercase}</Label>
                      <Switch
                        id="require_lowercase"
                        name="require_lowercase"
                        defaultChecked={settingsData?.security?.password_policy?.require_lowercase}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <Label htmlFor="require_number">{t.admin.requireNumber}</Label>
                      <Switch
                        id="require_number"
                        name="require_number"
                        defaultChecked={settingsData?.security?.password_policy?.require_number}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <Label htmlFor="require_special">{t.admin.requireSpecial}</Label>
                      <Switch
                        id="require_special"
                        name="require_special"
                        defaultChecked={settingsData?.security?.password_policy?.require_special}
                      />
                    </div>
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            {/* OAuth - Google */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Shield className="h-5 w-5" />
                  {t.admin.captchaSettings}
                </CardTitle>
                <CardDescription>
                  {t.admin.captchaSettingsDesc}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('security', {
                      captcha: {
                        provider: captchaProvider,
                        site_key: formData.get('captcha_site_key') || '',
                        secret_key: formData.get('captcha_secret_key') || '',
                        enable_for_login: formData.get('captcha_enable_login') === 'on',
                        enable_for_register: formData.get('captcha_enable_register') === 'on',
                        enable_for_serial_verify: formData.get('captcha_enable_serial_verify') === 'on',
                      },
                    })
                  }}
                  className="space-y-4"
                >
                  <div>
                    <Label>{t.admin.captchaProvider}</Label>
                    <Select value={captchaProvider} onValueChange={setCaptchaProvider}>
                      <SelectTrigger className="mt-1.5">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="none">{t.admin.captchaDisabled}</SelectItem>
                        <SelectItem value="cloudflare">Cloudflare Turnstile</SelectItem>
                        <SelectItem value="google">Google reCAPTCHA</SelectItem>
                        <SelectItem value="builtin">{t.admin.builtinCaptcha}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  {(captchaProvider === 'cloudflare' || captchaProvider === 'google') && (
                    <>
                      <div>
                        <Label htmlFor="captcha_site_key">Site Key</Label>
                        <Input
                          id="captcha_site_key"
                          name="captcha_site_key"
                          defaultValue={settingsData?.security?.captcha?.site_key || ''}
                          placeholder={captchaProvider === 'cloudflare' ? 'Turnstile Site Key' : 'reCAPTCHA Site Key'}
                          className="mt-1.5"
                        />
                      </div>
                      <div>
                        <Label htmlFor="captcha_secret_key">Secret Key</Label>
                        <Input
                          id="captcha_secret_key"
                          name="captcha_secret_key"
                          type="password"
                          defaultValue={settingsData?.security?.captcha?.secret_key || ''}
                          placeholder={captchaProvider === 'cloudflare' ? 'Turnstile Secret Key' : 'reCAPTCHA Secret Key'}
                          className="mt-1.5"
                        />
                      </div>
                    </>
                  )}

                  {captchaProvider !== 'none' && (
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <div>
                          <Label htmlFor="captcha_enable_login">{t.admin.loginCaptcha}</Label>
                          <p className="text-xs text-muted-foreground mt-1">
                            {t.admin.loginCaptchaHint}
                          </p>
                        </div>
                        <Switch
                          id="captcha_enable_login"
                          name="captcha_enable_login"
                          defaultChecked={settingsData?.security?.captcha?.enable_for_login}
                        />
                      </div>

                      <div className="flex items-center justify-between">
                        <div>
                          <Label htmlFor="captcha_enable_register">{t.admin.registerCaptcha}</Label>
                          <p className="text-xs text-muted-foreground mt-1">
                            {t.admin.registerCaptchaHint}
                          </p>
                        </div>
                        <Switch
                          id="captcha_enable_register"
                          name="captcha_enable_register"
                          defaultChecked={settingsData?.security?.captcha?.enable_for_register}
                        />
                      </div>

                      <div className="flex items-center justify-between">
                        <div>
                          <Label htmlFor="captcha_enable_serial_verify">{t.admin.serialVerifyCaptcha}</Label>
                          <p className="text-xs text-muted-foreground mt-1">
                            {t.admin.serialVerifyCaptchaHint}
                          </p>
                        </div>
                        <Switch
                          id="captcha_enable_serial_verify"
                          name="captcha_enable_serial_verify"
                          defaultChecked={settingsData?.security?.captcha?.enable_for_serial_verify}
                        />
                      </div>
                    </div>
                  )}

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            {/* OAuth - Google */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Globe className="h-5 w-5" />
                  Google OAuth
                </CardTitle>
                <CardDescription>{t.admin.googleOAuthDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('oauth', {
                      google: {
                        enabled: formData.get('google_enabled') === 'on',
                        client_id: formData.get('google_client_id'),
                        client_secret: formData.get('google_client_secret'),
                        redirect_url: formData.get('google_redirect_url'),
                      },
                    })
                  }}
                  className="space-y-4"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <Label htmlFor="google_enabled">{t.admin.enableGoogleLogin}</Label>
                      <p className="text-xs text-muted-foreground mt-1">
                        {t.admin.enableGoogleLoginHint}
                      </p>
                    </div>
                    <Switch
                      id="google_enabled"
                      name="google_enabled"
                      defaultChecked={settingsData?.oauth?.google?.enabled}
                    />
                  </div>

                  <div>
                    <Label htmlFor="google_client_id">Client ID</Label>
                    <Input
                      id="google_client_id"
                      name="google_client_id"
                      defaultValue={settingsData?.oauth?.google?.client_id || ''}
                      className="mt-1.5"
                    />
                  </div>

                  <div>
                    <Label htmlFor="google_client_secret">Client Secret</Label>
                    <Input
                      id="google_client_secret"
                      name="google_client_secret"
                      type="password"
                      placeholder={t.admin.passwordPlaceholder}
                      className="mt-1.5"
                    />
                  </div>

                  <div>
                    <Label htmlFor="google_redirect_url">{t.admin.callbackUrl}</Label>
                    <Input
                      id="google_redirect_url"
                      name="google_redirect_url"
                      defaultValue={settingsData?.oauth?.google?.redirect_url || ''}
                      className="mt-1.5"
                    />
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            {/* OAuth - GitHub */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Globe className="h-5 w-5" />
                  GitHub OAuth
                </CardTitle>
                <CardDescription>{t.admin.githubOAuthDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('oauth', {
                      github: {
                        enabled: formData.get('github_enabled') === 'on',
                        client_id: formData.get('github_client_id'),
                        client_secret: formData.get('github_client_secret'),
                        redirect_url: formData.get('github_redirect_url'),
                      },
                    })
                  }}
                  className="space-y-4"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <Label htmlFor="github_enabled">{t.admin.enableGithubLogin}</Label>
                      <p className="text-xs text-muted-foreground mt-1">
                        {t.admin.enableGithubLoginHint}
                      </p>
                    </div>
                    <Switch
                      id="github_enabled"
                      name="github_enabled"
                      defaultChecked={settingsData?.oauth?.github?.enabled}
                    />
                  </div>

                  <div>
                    <Label htmlFor="github_client_id">Client ID</Label>
                    <Input
                      id="github_client_id"
                      name="github_client_id"
                      defaultValue={settingsData?.oauth?.github?.client_id || ''}
                      className="mt-1.5"
                    />
                  </div>

                  <div>
                    <Label htmlFor="github_client_secret">Client Secret</Label>
                    <Input
                      id="github_client_secret"
                      name="github_client_secret"
                      type="password"
                      placeholder={t.admin.passwordPlaceholder}
                      className="mt-1.5"
                    />
                  </div>

                  <div>
                    <Label htmlFor="github_redirect_url">{t.admin.callbackUrl}</Label>
                    <Input
                      id="github_redirect_url"
                      name="github_redirect_url"
                      defaultValue={settingsData?.oauth?.github?.redirect_url || ''}
                      className="mt-1.5"
                    />
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* 限流设置 */}
        <TabsContent value="ratelimit">
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.rateLimitSettings}</CardTitle>
              <CardDescription>{t.admin.rateLimitSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  handleSubmit('rate_limit', {
                    enabled: formData.get('enabled') === 'on',
                    api: parseInt(formData.get('api') as string),
                    user_login: parseInt(formData.get('user_login') as string),
                    user_request: parseInt(formData.get('user_request') as string),
                    admin_request: parseInt(formData.get('admin_request') as string),
                  })
                }}
                className="space-y-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="rate_enabled">{t.admin.enableRateLimit}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableRateLimitHint}
                    </p>
                  </div>
                  <Switch
                    id="rate_enabled"
                    name="enabled"
                    defaultChecked={settingsData?.rate_limit?.enabled}
                  />
                </div>

                <div>
                  <Label htmlFor="api_limit">{t.admin.externalApiLimit}</Label>
                  <Input
                    id="api_limit"
                    name="api"
                    type="number"
                    defaultValue={settingsData?.rate_limit?.api || 10000}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="user_login_limit">{t.admin.userLoginLimit}</Label>
                  <Input
                    id="user_login_limit"
                    name="user_login"
                    type="number"
                    defaultValue={settingsData?.rate_limit?.user_login || 100}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="user_request_limit">{t.admin.userRequestLimit}</Label>
                  <Input
                    id="user_request_limit"
                    name="user_request"
                    type="number"
                    defaultValue={settingsData?.rate_limit?.user_request || 600}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="admin_request_limit">{t.admin.adminRequestLimit}</Label>
                  <Input
                    id="admin_request_limit"
                    name="admin_request"
                    type="number"
                    defaultValue={settingsData?.rate_limit?.admin_request || 2000}
                    className="mt-1.5"
                  />
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 上传设置 */}
        <TabsContent value="upload">
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.uploadSettings}</CardTitle>
              <CardDescription>{t.admin.uploadSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  const allowedTypes = (formData.get('allowed_types') as string).split(',').map(s => s.trim())
                  handleSubmit('upload', {
                    dir: formData.get('dir'),
                    max_size: parseInt(formData.get('max_size') as string) * 1024 * 1024,
                    allowed_types: allowedTypes,
                  })
                }}
                className="space-y-4"
              >
                <div>
                  <Label htmlFor="upload_dir">{t.admin.uploadDir}</Label>
                  <Input
                    id="upload_dir"
                    name="dir"
                    defaultValue={settingsData?.upload?.dir || 'uploads'}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="max_size">{t.admin.maxFileSize}</Label>
                  <Input
                    id="max_size"
                    name="max_size"
                    type="number"
                    defaultValue={(settingsData?.upload?.max_size || 5242880) / 1024 / 1024}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="allowed_types">{t.admin.allowedFileTypes}</Label>
                  <Input
                    id="allowed_types"
                    name="allowed_types"
                    defaultValue={settingsData?.upload?.allowed_types?.join(', ') || '.jpg, .jpeg, .png, .gif, .webp'}
                    placeholder=".jpg, .jpeg, .png"
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.separateWithComma}
                  </p>
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 高级设置 */}
        <TabsContent value="advanced">
          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle>{t.admin.logConfig}</CardTitle>
                <CardDescription>{t.admin.logConfigDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('log', {
                      level: formData.get('log_level'),
                      format: formData.get('log_format'),
                      output: formData.get('log_output'),
                      file_path: formData.get('log_file_path'),
                    })
                  }}
                  className="space-y-4"
                >
                  <div>
                    <Label htmlFor="log_level">{t.admin.logLevel}</Label>
                    <select
                      id="log_level"
                      name="log_level"
                      defaultValue={settingsData?.log?.level || 'info'}
                      className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                    >
                      <option value="debug">Debug</option>
                      <option value="info">Info</option>
                      <option value="warn">Warn</option>
                      <option value="error">Error</option>
                    </select>
                  </div>

                  <div>
                    <Label htmlFor="log_format">{t.admin.logFormat}</Label>
                    <select
                      id="log_format"
                      name="log_format"
                      defaultValue={settingsData?.log?.format || 'json'}
                      className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                    >
                      <option value="json">JSON</option>
                      <option value="text">Text</option>
                    </select>
                  </div>

                  <div>
                    <Label htmlFor="log_output">{t.admin.logOutput}</Label>
                    <select
                      id="log_output"
                      name="log_output"
                      defaultValue={settingsData?.log?.output || 'stdout'}
                      className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                    >
                      <option value="stdout">{t.admin.stdout}</option>
                      <option value="file">{t.admin.file}</option>
                      <option value="both">{t.admin.both}</option>
                    </select>
                  </div>

                  <div>
                    <Label htmlFor="log_file_path">{t.admin.logFilePath}</Label>
                    <Input
                      id="log_file_path"
                      name="log_file_path"
                      defaultValue={settingsData?.log?.file_path || 'logs/app.log'}
                      placeholder="logs/app.log"
                      className="mt-1.5"
                    />
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t.admin.corsConfig}</CardTitle>
                <CardDescription>{t.admin.corsConfigDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    const origins = (formData.get('allowed_origins') as string).split('\n').map(o => o.trim()).filter(Boolean)
                    handleSubmit('security', {
                      cors: {
                        allowed_origins: origins,
                        max_age: parseInt(formData.get('cors_max_age') as string),
                      },
                    })
                  }}
                  className="space-y-4"
                >
                  <div>
                    <Label htmlFor="allowed_origins">{t.admin.allowedOrigins}</Label>
                    <textarea
                      id="allowed_origins"
                      name="allowed_origins"
                      defaultValue={settingsData?.security?.cors?.allowed_origins?.join('\n') || ''}
                      placeholder={"http://localhost:3000\nhttps://example.com"}
                      rows={5}
                      className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                    />
                  </div>

                  <div>
                    <Label htmlFor="cors_max_age">{t.admin.preflightCacheTime}</Label>
                    <Input
                      id="cors_max_age"
                      name="cors_max_age"
                      type="number"
                      defaultValue={settingsData?.security?.cors?.max_age || 86400}
                      className="mt-1.5"
                    />
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <ShieldCheck className="h-5 w-5" />
                  {t.admin.ipHeaderConfig}
                </CardTitle>
                <CardDescription>
                  {t.admin.ipHeaderConfigDesc}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    const tpRaw = (formData.get('trusted_proxies') as string) || ''
                    const trustedProxies = tpRaw
                      .split(/\r?\n/)
                      .map((x) => x.trim())
                      .filter(Boolean)
                    handleSubmit('security', {
                      ip_header: formData.get('ip_header') || '',
                      ip_header_submitted: true,
                      trusted_proxies: trustedProxies,
                      trusted_proxies_submitted: true,
                    })
                  }}
                  className="space-y-4"
                >
                  <div>
                    <Label htmlFor="ip_header">{t.admin.ipHeaderName}</Label>
                    <Input
                      id="ip_header"
                      name="ip_header"
                      defaultValue={settingsData?.security?.ip_header || ''}
                      placeholder="CF-Connecting-IP, X-Real-IP, X-Forwarded-For"
                      className="mt-1.5"
                    />
                    <p className="text-xs text-muted-foreground mt-2">
                      {t.admin.ipHeaderHint}
                    </p>
                    <ul className="text-xs text-muted-foreground mt-1 ml-4 list-disc space-y-0.5">
                      <li><code className="bg-muted px-1 py-0.5 rounded">CF-Connecting-IP</code> - Cloudflare</li>
                      <li><code className="bg-muted px-1 py-0.5 rounded">X-Real-IP</code> - Nginx</li>
                      <li><code className="bg-muted px-1 py-0.5 rounded">X-Forwarded-For</code> - {t.admin.standardProxy}</li>
                    </ul>
                  </div>

                  <div>
                    <Label htmlFor="trusted_proxies">{t.admin.trustedProxies}</Label>
                    <textarea
                      id="trusted_proxies"
                      name="trusted_proxies"
                      defaultValue={(settingsData?.security?.trusted_proxies || []).join('\n')}
                      placeholder={t.admin.trustedProxiesPlaceholder}
                      rows={5}
                      className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
                    />
                    <p className="text-xs text-muted-foreground mt-2">
                      {t.admin.trustedProxiesHint}
                    </p>
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {t.admin.saveSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t.admin.redisConfigReadonly}</CardTitle>
                <CardDescription>{t.admin.redisConfigReadonlyDesc}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div>
                  <Label>{t.admin.host}</Label>
                  <Input
                    value={settingsData?.redis?.host || ''}
                    disabled
                    className="mt-1.5"
                  />
                </div>
                <div>
                  <Label>{t.admin.port}</Label>
                  <Input
                    value={settingsData?.redis?.port || ''}
                    disabled
                    className="mt-1.5"
                  />
                </div>
                <div>
                  <Label>{t.admin.database}</Label>
                  <Input
                    value={settingsData?.redis?.db || ''}
                    disabled
                    className="mt-1.5"
                  />
                </div>
                <p className="text-xs text-muted-foreground">
                  {t.admin.redisConfigHint}
                </p>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* 工单设置 */}
        <TabsContent value="ticket">
          <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.ticketSettings}</CardTitle>
              <CardDescription>{t.admin.ticketSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  const categories = (formData.get('categories') as string).split('\n').map(c => c.trim()).filter(Boolean)
                  handleSubmit('ticket', {
                    enabled: formData.get('enabled') === 'on',
                    categories: categories,
                    template: formData.get('template'),
                    max_content_length: parseInt(formData.get('max_content_length') as string) || 0,
                    auto_close_hours: parseInt(formData.get('auto_close_hours') as string) || 0,
                  })
                }}
                className="space-y-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="ticket_enabled">{t.admin.enableTicketSystem}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableTicketSystemHint}
                    </p>
                  </div>
                  <Switch
                    id="ticket_enabled"
                    name="enabled"
                    defaultChecked={settingsData?.ticket?.enabled}
                  />
                </div>

                <div>
                  <Label htmlFor="ticket_categories">{t.admin.ticketCategories}</Label>
                  <textarea
                    id="ticket_categories"
                    name="categories"
                    defaultValue={settingsData?.ticket?.categories?.join('\n') || '订单问题\n支付问题\n售后服务\n技术支持\n其他问题'}
                    placeholder={"订单问题\n支付问题\n售后服务"}
                    rows={5}
                    className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.ticketCategoriesHint}
                  </p>
                </div>

                <div>
                  <Label htmlFor="ticket_template">{t.admin.ticketTemplate}</Label>
                  <textarea
                    id="ticket_template"
                    name="template"
                    defaultValue={settingsData?.ticket?.template || ''}
                    placeholder={"请描述您的问题：\n\n相关订单号（如有）：\n\n期望的解决方案："}
                    rows={8}
                    className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.ticketTemplateHint}
                  </p>
                </div>

                <div>
                  <Label htmlFor="max_content_length">{t.admin.maxContentLength}</Label>
                  <Input
                    id="max_content_length"
                    name="max_content_length"
                    type="number"
                    min="0"
                    max="100000"
                    defaultValue={settingsData?.ticket?.max_content_length || 0}
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.maxContentLengthHint}
                  </p>
                </div>

                <div>
                  <Label htmlFor="auto_close_hours">{t.admin.autoCloseHours}</Label>
                  <Input
                    id="auto_close_hours"
                    name="auto_close_hours"
                    type="number"
                    min="0"
                    max="8760"
                    defaultValue={settingsData?.ticket?.auto_close_hours || 0}
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.autoCloseHoursHint}
                  </p>
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>

          {/* 工单附件设置 */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FileUp className="h-5 w-5" />
                {t.admin.ticketAttachmentSettings}
              </CardTitle>
              <CardDescription>{t.admin.ticketAttachmentSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  const allowedImageTypes = (formData.get('allowed_image_types') as string).split(',').map(s => s.trim()).filter(Boolean)
                  handleSubmit('ticket', {
                    attachment: {
                      enable_image: formData.get('enable_image') === 'on',
                      enable_voice: formData.get('enable_voice') === 'on',
                      max_image_size: parseInt(formData.get('max_image_size') as string) * 1024 * 1024,
                      max_voice_size: parseInt(formData.get('max_voice_size') as string) * 1024 * 1024,
                      max_voice_duration: parseInt(formData.get('max_voice_duration') as string),
                      allowed_image_types: allowedImageTypes,
                      retention_days: parseInt(formData.get('retention_days') as string) || 0,
                    },
                  })
                }}
                className="space-y-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="enable_image">{t.admin.enableImageUpload}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableImageUploadHint}
                    </p>
                  </div>
                  <Switch
                    id="enable_image"
                    name="enable_image"
                    defaultChecked={settingsData?.ticket?.attachment?.enable_image ?? true}
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="enable_voice">{t.admin.enableVoiceUpload}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableVoiceUploadHint}
                    </p>
                  </div>
                  <Switch
                    id="enable_voice"
                    name="enable_voice"
                    defaultChecked={settingsData?.ticket?.attachment?.enable_voice ?? true}
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <Label htmlFor="max_image_size">{t.admin.maxImageSize}</Label>
                    <Input
                      id="max_image_size"
                      name="max_image_size"
                      type="number"
                      min="1"
                      max="50"
                      defaultValue={(settingsData?.ticket?.attachment?.max_image_size || 5242880) / 1024 / 1024}
                      className="mt-1.5"
                    />
                  </div>
                  <div>
                    <Label htmlFor="max_voice_size">{t.admin.maxVoiceSize}</Label>
                    <Input
                      id="max_voice_size"
                      name="max_voice_size"
                      type="number"
                      min="1"
                      max="50"
                      defaultValue={(settingsData?.ticket?.attachment?.max_voice_size || 10485760) / 1024 / 1024}
                      className="mt-1.5"
                    />
                  </div>
                </div>

                <div>
                  <Label htmlFor="max_voice_duration">{t.admin.maxVoiceDuration}</Label>
                  <Input
                    id="max_voice_duration"
                    name="max_voice_duration"
                    type="number"
                    min="10"
                    max="300"
                    defaultValue={settingsData?.ticket?.attachment?.max_voice_duration || 60}
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.maxVoiceDurationHint}
                  </p>
                </div>

                <div>
                  <Label htmlFor="allowed_image_types">{t.admin.allowedImageFormats}</Label>
                  <Input
                    id="allowed_image_types"
                    name="allowed_image_types"
                    defaultValue={settingsData?.ticket?.attachment?.allowed_image_types?.join(', ') || '.jpg, .jpeg, .png, .gif, .webp'}
                    placeholder=".jpg, .jpeg, .png, .gif, .webp"
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.separateWithCommaFormats}
                  </p>
                </div>

                <div>
                  <Label htmlFor="retention_days">{t.admin.attachmentRetentionDays}</Label>
                  <Input
                    id="retention_days"
                    name="retention_days"
                    type="number"
                    min="0"
                    max="3650"
                    defaultValue={settingsData?.ticket?.attachment?.retention_days || 0}
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.attachmentRetentionDaysHint}
                  </p>
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveAttachmentSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
          </div>
        </TabsContent>

        {/* 序列号查询设置 */}
        <TabsContent value="serial">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <ShieldCheck className="h-5 w-5" />
                {t.admin.serialSettings}
              </CardTitle>
              <CardDescription>{t.admin.serialSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  handleSubmit('serial', {
                    _submitted: true,
                    enabled: formData.get('enabled') === 'on',
                  })
                }}
                className="space-y-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="serial_enabled">{t.admin.enableSerialQuery}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableSerialQueryHint}
                    </p>
                  </div>
                  <Switch
                    id="serial_enabled"
                    name="enabled"
                    defaultChecked={settingsData?.serial?.enabled}
                  />
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 数据分析设置 */}
        <TabsContent value="analytics">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <BarChart3 className="h-5 w-5" />
                {t.admin.analyticsSettings}
              </CardTitle>
              <CardDescription>{t.admin.analyticsSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  handleSubmit('analytics', {
                    _submitted: true,
                    enabled: formData.get('enabled') === 'on',
                  })
                }}
                className="space-y-4"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <Label htmlFor="analytics_enabled">{t.admin.enableAnalytics}</Label>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.enableAnalyticsHint}
                    </p>
                  </div>
                  <Switch
                    id="analytics_enabled"
                    name="enabled"
                    defaultChecked={settingsData?.analytics?.enabled}
                  />
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 订单设置 */}
        <TabsContent value="order">
          <Card>
            <CardHeader>
              <CardTitle>{t.admin.orderSettings}</CardTitle>
              <CardDescription>{t.admin.orderSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  handleSubmit('order', {
                    no_prefix: formData.get('no_prefix'),
                    auto_cancel_hours: parseInt(formData.get('auto_cancel_hours') as string),
                    currency: formData.get('currency'),
                    virtual_delivery_order: formData.get('virtual_delivery_order'),
                    stock_display: {
                      mode: formData.get('stock_display_mode'),
                      low_stock_threshold: parseInt(formData.get('low_stock_threshold') as string) || 10,
                      high_stock_threshold: parseInt(formData.get('high_stock_threshold') as string) || 50,
                    },
                  })
                }}
                className="space-y-4"
              >
                <div>
                  <Label htmlFor="no_prefix">{t.admin.orderNoPrefix}</Label>
                  <Input
                    id="no_prefix"
                    name="no_prefix"
                    defaultValue={settingsData?.order?.no_prefix || 'ORD'}
                    placeholder="ORD"
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.orderNoPrefixExample}
                  </p>
                </div>

                <div>
                  <Label htmlFor="currency">{t.admin.currency}</Label>
                  <select
                    id="currency"
                    name="currency"
                    defaultValue={settingsData?.order?.currency || 'CNY'}
                    className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                  >
                    <option value="CNY">{t.admin.currencyCNY}</option>
                    <option value="USD">{t.admin.currencyUSD}</option>
                    <option value="EUR">{t.admin.currencyEUR}</option>
                    <option value="JPY">{t.admin.currencyJPY}</option>
                    <option value="GBP">{t.admin.currencyGBP}</option>
                    <option value="KRW">{t.admin.currencyKRW}</option>
                    <option value="HKD">{t.admin.currencyHKD}</option>
                    <option value="TWD">{t.admin.currencyTWD}</option>
                    <option value="SGD">{t.admin.currencySGD}</option>
                    <option value="AUD">{t.admin.currencyAUD}</option>
                    <option value="CAD">{t.admin.currencyCAD}</option>
                  </select>
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.currencyHint}
                  </p>
                </div>

                <div>
                  <Label htmlFor="auto_cancel_hours">{t.admin.autoCancelHours}</Label>
                  <Input
                    id="auto_cancel_hours"
                    name="auto_cancel_hours"
                    type="number"
                    defaultValue={settingsData?.order?.auto_cancel_hours || 72}
                    className="mt-1.5"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    {t.admin.autoCancelHoursHint}
                  </p>
                </div>

                <div className="border-t border-border pt-4 mt-4">
                  <h4 className="font-medium mb-3">{t.admin.virtualDeliveryOrderTitle}</h4>
                  <div>
                    <Label htmlFor="virtual_delivery_order">{t.admin.virtualDeliveryOrder}</Label>
                    <select
                      id="virtual_delivery_order"
                      name="virtual_delivery_order"
                      defaultValue={settingsData?.order?.virtual_delivery_order || 'random'}
                      className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                    >
                      <option value="random">{t.admin.virtualDeliveryRandom}</option>
                      <option value="newest">{t.admin.virtualDeliveryNewest}</option>
                      <option value="oldest">{t.admin.virtualDeliveryOldest}</option>
                    </select>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.virtualDeliveryOrderHint}
                    </p>
                  </div>
                </div>

                <div className="border-t border-border pt-4 mt-4">
                  <h4 className="font-medium mb-3">{t.admin.stockDisplayTitle}</h4>
                  <div className="space-y-4">
                    <div>
                      <Label htmlFor="stock_display_mode">{t.admin.stockDisplayMode}</Label>
                      <select
                        id="stock_display_mode"
                        name="stock_display_mode"
                        defaultValue={settingsData?.order?.stock_display?.mode || 'exact'}
                        className="mt-1.5 w-full rounded-md border border-input bg-background px-3 py-2"
                      >
                        <option value="exact">{t.admin.stockDisplayModeExact}</option>
                        <option value="level">{t.admin.stockDisplayModeLevel}</option>
                        <option value="hidden">{t.admin.stockDisplayModeHidden}</option>
                      </select>
                      <p className="text-xs text-muted-foreground mt-1">
                        {settingsData?.order?.stock_display?.mode === 'level'
                          ? t.admin.stockDisplayModeLevelDesc
                          : settingsData?.order?.stock_display?.mode === 'hidden'
                            ? t.admin.stockDisplayModeHiddenDesc
                            : t.admin.stockDisplayModeExactDesc}
                      </p>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <Label htmlFor="low_stock_threshold">{t.admin.stockLowThreshold}</Label>
                        <Input
                          id="low_stock_threshold"
                          name="low_stock_threshold"
                          type="number"
                          min="0"
                          defaultValue={settingsData?.order?.stock_display?.low_stock_threshold || 10}
                          className="mt-1.5"
                        />
                        <p className="text-xs text-muted-foreground mt-1">
                          {t.admin.stockLowThresholdHint}
                        </p>
                      </div>
                      <div>
                        <Label htmlFor="high_stock_threshold">{t.admin.stockHighThreshold}</Label>
                        <Input
                          id="high_stock_threshold"
                          name="high_stock_threshold"
                          type="number"
                          min="0"
                          defaultValue={settingsData?.order?.stock_display?.high_stock_threshold || 50}
                          className="mt-1.5"
                        />
                        <p className="text-xs text-muted-foreground mt-1">
                          {t.admin.stockHighThresholdHint}
                        </p>
                      </div>
                    </div>
                  </div>
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>

          <Card className="mt-4">
            <CardHeader>
              <CardTitle>{t.admin.formAndLinkSettings}</CardTitle>
              <CardDescription>{t.admin.formAndLinkSettingsDesc}</CardDescription>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => {
                  e.preventDefault()
                  const formData = new FormData(e.currentTarget)
                  const magicData = {
                    expire_minutes: parseInt(formData.get('magic_expire_minutes') as string),
                    max_uses: parseInt(formData.get('magic_max_uses') as string),
                  }
                  const formConfigData = {
                    expire_hours: parseInt(formData.get('form_expire_hours') as string),
                  }

                  // 分别提交
                  updateMutation.mutate({
                    magic_link: magicData,
                    form: formConfigData,
                  })
                }}
                className="space-y-4"
              >
                <div>
                  <Label htmlFor="magic_expire_minutes">{t.admin.magicLinkExpiry}</Label>
                  <Input
                    id="magic_expire_minutes"
                    name="magic_expire_minutes"
                    type="number"
                    defaultValue={settingsData?.magic_link?.expire_minutes || 15}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="magic_max_uses">{t.admin.magicLinkMaxUses}</Label>
                  <Input
                    id="magic_max_uses"
                    name="magic_max_uses"
                    type="number"
                    defaultValue={settingsData?.magic_link?.max_uses || 1}
                    className="mt-1.5"
                  />
                </div>

                <div>
                  <Label htmlFor="form_expire_hours">{t.admin.formExpiry}</Label>
                  <Input
                    id="form_expire_hours"
                    name="form_expire_hours"
                    type="number"
                    defaultValue={settingsData?.form?.expire_hours || 24}
                    className="mt-1.5"
                  />
                </div>

                <Button type="submit" disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {t.admin.saveSettings}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        {/* 个性化设置 */}
        <TabsContent value="personalization">
          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Palette className="h-5 w-5" />
                  {t.admin.themeColor}
                </CardTitle>
                <CardDescription>{t.admin.themeColorDesc}</CardDescription>
              </CardHeader>
              <CardContent>
                <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    const formData = new FormData(e.currentTarget)
                    handleSubmit('customization', {
                      _submitted: true,
                      primary_color: primaryColor,
                      logo_url: formData.get('logo_url') || '',
                      favicon_url: formData.get('favicon_url') || '',
                      page_rules: pageRules,
                    })
                  }}
                  className="space-y-4"
                >
                  <div>
                    <Label>{t.admin.primaryColorLabel}</Label>
                    <Select value={primaryColor} onValueChange={setPrimaryColor}>
                      <SelectTrigger className="mt-1.5">
                        <SelectValue placeholder={t.admin.selectPrimaryColor} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="217.2 91% 60%">
                          <div className="flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full" style={{ background: 'hsl(217.2, 91%, 60%)' }} />
                            {t.admin.blueDefault}
                          </div>
                        </SelectItem>
                        <SelectItem value="142.1 76.2% 36.3%">
                          <div className="flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full" style={{ background: 'hsl(142.1, 76.2%, 36.3%)' }} />
                            {t.admin.green}
                          </div>
                        </SelectItem>
                        <SelectItem value="346.8 77.2% 49.8%">
                          <div className="flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full" style={{ background: 'hsl(346.8, 77.2%, 49.8%)' }} />
                            {t.admin.rose}
                          </div>
                        </SelectItem>
                        <SelectItem value="262.1 83.3% 57.8%">
                          <div className="flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full" style={{ background: 'hsl(262.1, 83.3%, 57.8%)' }} />
                            {t.admin.purple}
                          </div>
                        </SelectItem>
                        <SelectItem value="24.6 95% 53.1%">
                          <div className="flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full" style={{ background: 'hsl(24.6, 95%, 53.1%)' }} />
                            {t.admin.orange}
                          </div>
                        </SelectItem>
                        <SelectItem value="0 72.2% 50.6%">
                          <div className="flex items-center gap-2">
                            <div className="w-4 h-4 rounded-full" style={{ background: 'hsl(0, 72.2%, 50.6%)' }} />
                            {t.admin.red}
                          </div>
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.primaryColorHint}
                    </p>
                  </div>

                  <div>
                    <Label htmlFor="logo_url">Logo URL</Label>
                    <Input
                      id="logo_url"
                      name="logo_url"
                      defaultValue={settingsData?.customization?.logo_url || ''}
                      placeholder="https://example.com/logo.png"
                      className="mt-1.5"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.logoUrlHint}
                    </p>
                  </div>

                  <div>
                    <Label htmlFor="favicon_url">Favicon URL</Label>
                    <Input
                      id="favicon_url"
                      name="favicon_url"
                      defaultValue={settingsData?.customization?.favicon_url || ''}
                      placeholder="https://example.com/favicon.ico"
                      className="mt-1.5"
                    />
                    <p className="text-xs text-muted-foreground mt-1">
                      {t.admin.faviconUrlHint}
                    </p>
                  </div>

                  <Button type="submit" disabled={updateMutation.isPending}>
                    <Save className="mr-2 h-4 w-4" />
                    {updateMutation.isPending ? t.admin.saving : t.admin.saveColorSettings}
                  </Button>
                </form>
              </CardContent>
            </Card>

            {/* 页面定向规则 */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <FileCode className="h-5 w-5" />
                  {t.admin.pageRules}
                </CardTitle>
                <CardDescription>{t.admin.pageRulesDesc}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {/* 内置规则快捷添加 */}
                <div>
                  <Label>{t.admin.quickAddBuiltinRules}</Label>
                  <div className="flex flex-wrap gap-2 mt-1.5">
                    {[
                      { label: t.admin.presetGlobal, name: t.admin.presetGlobalName, pattern: '.*', match_type: 'regex' },
                      { label: t.admin.presetHome, name: t.admin.presetHomeName, pattern: '/', match_type: 'exact' },
                      { label: t.admin.presetProducts, name: t.admin.presetProductsName, pattern: '/products', match_type: 'exact' },
                      { label: t.admin.presetProductDetail, name: t.admin.presetProductDetailName, pattern: '^/products/[^/]+$', match_type: 'regex' },
                      { label: t.admin.presetCart, name: t.admin.presetCartName, pattern: '/cart', match_type: 'exact' },
                      { label: t.admin.presetOrders, name: t.admin.presetOrdersName, pattern: '^/orders', match_type: 'regex' },
                      { label: t.admin.presetLogin, name: t.admin.presetLoginName, pattern: '/login', match_type: 'exact' },
                      { label: t.admin.presetTickets, name: t.admin.presetTicketsName, pattern: '^/tickets', match_type: 'regex' },
                      { label: t.admin.presetAdmin, name: t.admin.presetAdminName, pattern: '^/admin', match_type: 'regex' },
                    ].map((preset) => (
                      <Button
                        key={preset.pattern}
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setPageRules(prev => [...prev, {
                            name: preset.name,
                            pattern: preset.pattern,
                            match_type: preset.match_type,
                            css: '',
                            js: '',
                            enabled: true,
                          }])
                        }}
                      >
                        <Plus className="mr-1 h-3 w-3" />
                        {preset.label}
                      </Button>
                    ))}
                  </div>
                </div>

                {/* 自定义添加 */}
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setPageRules(prev => [...prev, {
                      name: '',
                      pattern: '',
                      match_type: 'exact',
                      css: '',
                      js: '',
                      enabled: true,
                    }])
                  }}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  {t.admin.addCustomRule}
                </Button>

                {/* 规则列表 */}
                {pageRules.map((rule, index) => (
                  <PageRuleItem
                    key={index}
                    rule={rule}
                    index={index}
                    onChange={handlePageRuleUpdate}
                    onDelete={handlePageRuleDelete}
                    t={t}
                  />
                ))}

                {pageRules.length === 0 && (
                  <p className="text-sm text-muted-foreground text-center py-4">
                    {t.admin.noPageRules}
                  </p>
                )}

                <Button
                  type="button"
                  disabled={updateMutation.isPending}
                  onClick={() => {
                    handleSubmit('customization', {
                      _submitted: true,
                      primary_color: primaryColor,
                      logo_url: settingsData?.customization?.logo_url || '',
                      favicon_url: settingsData?.customization?.favicon_url || '',
                      page_rules: pageRules,
                    })
                  }}
                >
                  <Save className="mr-2 h-4 w-4" />
                  {updateMutation.isPending ? t.admin.saving : t.admin.savePageRules}
                </Button>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        {/* 落地页编辑 */}
        <TabsContent value="landingpage">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t.admin.landingPage}</CardTitle>
              <CardDescription>{t.admin.landingPageDesc}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="rounded-md border bg-muted/30 p-3 text-sm space-y-1">
                <p className="font-medium">{t.admin.landingPageVariables}</p>
                <p className="text-muted-foreground text-xs">{t.admin.landingPageVariablesDesc}</p>
                <div className="flex flex-wrap gap-2 mt-2">
                  {['{{.AppName}}', '{{.AppURL}}', '{{.Currency}}', '{{.LogoURL}}', '{{.PrimaryColor}}', '{{.Year}}'].map(v => (
                    <Badge key={v} variant="secondary" className="font-mono text-xs">{v}</Badge>
                  ))}
                </div>
              </div>
              <div>
                <Label>{t.admin.landingPageHtml}</Label>
                <textarea
                  value={landingHtml}
                  onChange={(e) => setLandingHtml(e.target.value)}
                  className="mt-1 w-full min-h-[500px] p-3 font-mono text-sm border rounded-md bg-muted/50 focus:outline-none focus:ring-2 focus:ring-ring resize-y"
                  spellCheck={false}
                />
              </div>
              <div className="flex items-center gap-2">
                <Button
                  onClick={() => saveLandingPageMutation.mutate(landingHtml)}
                  disabled={saveLandingPageMutation.isPending}
                >
                  <Save className="mr-2 h-4 w-4" />
                  {saveLandingPageMutation.isPending ? t.admin.saving : t.admin.saveSettings}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => window.open('/', '_blank')}
                >
                  <Globe className="mr-2 h-4 w-4" />
                  {t.admin.landingPagePreview}
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => {
                    if (confirm(t.admin.landingPageResetConfirm)) {
                      resetLandingPageMutation.mutate()
                    }
                  }}
                  disabled={resetLandingPageMutation.isPending}
                >
                  <RotateCcw className="mr-2 h-4 w-4" />
                  {resetLandingPageMutation.isPending ? t.admin.saving : t.admin.landingPageReset}
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* 重要提示 */}
      <Card className="border-yellow-500/30 bg-yellow-500/10">
        <CardHeader>
          <CardTitle className="text-base">{t.admin.importantNotice}</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <p>• {t.admin.noticeRestart}</p>
          <p>• {t.admin.noticeManualEdit}</p>
          <p>• {t.admin.noticeCaution}</p>
          <p>• {t.admin.noticeBackup}<code className="text-xs bg-muted px-2 py-1 rounded">backend/config/config.json</code></p>
        </CardContent>
      </Card>
    </div>
  )
}
