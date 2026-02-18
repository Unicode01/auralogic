'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getPaymentMethods,
  createPaymentMethod,
  updatePaymentMethod,
  deletePaymentMethod,
  togglePaymentMethodEnabled,
  reorderPaymentMethods,
  testPaymentScript,
  initBuiltinPaymentMethods,
  PaymentMethod,
} from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { SandboxedHtmlFrame } from '@/components/ui/sandboxed-html-frame'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Plus,
  Pencil,
  Trash2,
  GripVertical,
  Play,
  CreditCard,
  Building2,
  Wallet,
  MessageCircle,
  Bitcoin,
  Code,
  Settings,
  RefreshCw,
  Coins,
  X,
} from 'lucide-react'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

const iconMap: Record<string, any> = {
  CreditCard,
  Building2,
  Wallet,
  MessageCircle,
  Bitcoin,
  Code,
  Coins,
}

type ConfigEntry = { key: string; value: any }

function numToPlain(n: number): string {
  const s = String(n)
  if (!s.includes('e') && !s.includes('E')) return s
  return n.toFixed(20).replace(/\.?0+$/, '')
}

function detectType(v: any): 'string' | 'number' | 'boolean' {
  if (typeof v === 'boolean') return 'boolean'
  if (typeof v === 'number') return 'number'
  return 'string'
}

function ConfigEditor({ value, onChange, flushRef, t }: { value: string; onChange: (v: string) => void; flushRef?: React.MutableRefObject<(() => string | null) | null>; t: ReturnType<typeof getTranslations> }) {
  const [entries, setEntries] = useState<ConfigEntry[]>([])
  const [rawMode, setRawMode] = useState(false)
  const [rawValue, setRawValue] = useState(value)
  const skipSync = useRef(false)
  const [rawNumbers, setRawNumbers] = useState<Record<number, string>>({})
  const debounceRef = useRef<ReturnType<typeof setTimeout>>()
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange
  const pendingJsonRef = useRef<string | null>(null)

  useEffect(() => () => { clearTimeout(debounceRef.current) }, [])

  // Expose flush method to parent
  useEffect(() => {
    if (flushRef) {
      flushRef.current = () => {
        clearTimeout(debounceRef.current)
        if (pendingJsonRef.current !== null) {
          const pending = pendingJsonRef.current
          onChangeRef.current(pending)
          pendingJsonRef.current = null
          return pending
        }
        return null
      }
    }
  }, [flushRef])

  // Parse JSON → entries when value changes externally
  useEffect(() => {
    if (skipSync.current) { skipSync.current = false; return }
    try {
      const obj = JSON.parse(value || '{}')
      setEntries(Object.entries(obj).map(([k, v]) => ({ key: k, value: v })))
      setRawValue(value)
    } catch {
      setRawMode(true)
      setRawValue(value)
    }
  }, [value])

  const sync = useCallback((next: ConfigEntry[]) => {
    setEntries(next)
    skipSync.current = true
    const lines = next.filter(e => e.key).map(e => {
      const v = typeof e.value === 'number' ? numToPlain(e.value) : JSON.stringify(e.value)
      return `  ${JSON.stringify(e.key)}: ${v}`
    })
    const json = lines.length ? '{\n' + lines.join(',\n') + '\n}' : '{}'
    setRawValue(json)
    pendingJsonRef.current = json
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      onChangeRef.current(json)
      pendingJsonRef.current = null
    }, 300)
  }, [])

  const updateEntry = (i: number, field: 'key' | 'value', val: any) => {
    const next = [...entries]
    next[i] = { ...next[i], [field]: val }
    sync(next)
  }

  const removeEntry = (i: number) => sync(entries.filter((_, idx) => idx !== i))

  const addEntry = () => sync([...entries, { key: '', value: '' }])

  const changeType = (i: number, type: string) => {
    const next = [...entries]
    const cur = next[i].value
    if (type === 'boolean') next[i] = { ...next[i], value: cur === 'true' || cur === true }
    else if (type === 'number') next[i] = { ...next[i], value: Number(cur) || 0 }
    else next[i] = { ...next[i], value: String(cur) }
    sync(next)
  }

  // Raw JSON mode
  if (rawMode) {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>{t.admin.pmConfigJson}</Label>
          <Button variant="ghost" size="sm" onClick={() => {
            try {
              const obj = JSON.parse(rawValue || '{}')
              setEntries(Object.entries(obj).map(([k, v]) => ({ key: k, value: v })))
              skipSync.current = true
              onChange(rawValue)
              setRawMode(false)
            } catch {
              toast.error(t.admin.pmInvalidJson)
            }
          }}>
            {t.admin.pmVisualEditor}
          </Button>
        </div>
        <Textarea
          value={rawValue}
          onChange={(e) => { setRawValue(e.target.value); onChange(e.target.value) }}
          rows={10}
          className="font-mono text-sm"
        />
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label>{t.admin.pmConfigFields}</Label>
        <Button variant="ghost" size="sm" onClick={() => setRawMode(true)}>
          {t.admin.pmJsonEditor}
        </Button>
      </div>

      {entries.length === 0 && (
        <p className="text-sm text-muted-foreground py-4 text-center">
          {t.admin.pmNoFields}
        </p>
      )}

      <div className="space-y-2">
        {entries.map((entry, i) => {
          const type = detectType(entry.value)
          return (
            <div key={i} className="flex items-center gap-2 rounded-lg border p-2 bg-muted/30">
              <Input
                value={entry.key}
                onChange={(e) => updateEntry(i, 'key', e.target.value)}
                placeholder="key"
                className="font-mono text-sm w-[35%]"
              />
              <Select value={type} onValueChange={(v) => changeType(i, v)}>
                <SelectTrigger className="w-[90px] text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="string">String</SelectItem>
                  <SelectItem value="number">Number</SelectItem>
                  <SelectItem value="boolean">Bool</SelectItem>
                </SelectContent>
              </Select>
              {type === 'boolean' ? (
                <div className="flex-1 flex items-center gap-2 pl-2">
                  <Switch
                    checked={!!entry.value}
                    onCheckedChange={(v) => updateEntry(i, 'value', v)}
                  />
                  <span className="text-xs text-muted-foreground">{String(entry.value)}</span>
                </div>
              ) : type === 'number' ? (
                <Input
                  value={rawNumbers[i] ?? numToPlain(entry.value)}
                  onFocus={() => setRawNumbers(p => ({ ...p, [i]: numToPlain(entry.value) }))}
                  onChange={(e) => {
                    const raw = e.target.value
                    if (raw === '' || /^-?\d*\.?\d*$/.test(raw)) {
                      setRawNumbers(p => ({ ...p, [i]: raw }))
                      const n = Number(raw)
                      if (raw !== '' && !isNaN(n)) updateEntry(i, 'value', n)
                    }
                  }}
                  onBlur={() => setRawNumbers(p => { const { [i]: _, ...rest } = p; return rest })}
                  className="flex-1 font-mono text-sm"
                />
              ) : (
                <Input
                  value={String(entry.value ?? '')}
                  onChange={(e) => updateEntry(i, 'value', e.target.value)}
                  className="flex-1 font-mono text-sm"
                  placeholder="value"
                />
              )}
              <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => removeEntry(i)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          )
        })}
      </div>

      <Button variant="outline" size="sm" onClick={addEntry} className="w-full">
        <Plus className="h-4 w-4 mr-1" />
        {t.admin.pmAddField}
      </Button>
    </div>
  )
}

export default function PaymentMethodsPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminPaymentMethods)
  const queryClient = useQueryClient()
  const [editingMethod, setEditingMethod] = useState<PaymentMethod | null>(null)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [testResult, setTestResult] = useState<string | null>(null)
  const configFlushRef = useRef<(() => string | null) | null>(null)

  // 页面加载时清除缓存并重新获取
  useEffect(() => {
    queryClient.removeQueries({ queryKey: ['paymentMethods'] })
  }, [])

  // 表单状态
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    type: 'custom' as 'custom',
    icon: 'CreditCard',
    script: '',
    config: '{}',
    poll_interval: 30,
  })

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['paymentMethods'],
    queryFn: () => getPaymentMethods(),
    staleTime: 0, // 数据立即过期，每次都重新获取
    refetchOnMount: 'always', // 组件挂载时总是重新获取
  })

  const methods = data?.data?.items || []

  const createMutation = useMutation({
    mutationFn: createPaymentMethod,
    onSuccess: () => {
      toast.success(t.admin.pmCreatedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      setIsCreateOpen(false)
      resetForm()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: Partial<PaymentMethod> }) =>
      updatePaymentMethod(id, data),
    onSuccess: () => {
      toast.success(t.admin.pmUpdatedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      setEditingMethod(null)
      resetForm()
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deletePaymentMethod,
    onSuccess: () => {
      toast.success(t.admin.pmDeletedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      setDeleteId(null)
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const toggleMutation = useMutation({
    mutationFn: togglePaymentMethodEnabled,
    onMutate: async (id) => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: ['paymentMethods'] })

      // Snapshot the previous value
      const previousData = queryClient.getQueryData(['paymentMethods'])

      // Optimistically update to the new value
      queryClient.setQueryData(['paymentMethods'], (old: any) => {
        if (!old?.data?.items) return old
        return {
          ...old,
          data: {
            ...old.data,
            items: old.data.items.map((method: PaymentMethod) =>
              method.id === id ? { ...method, enabled: !method.enabled } : method
            )
          }
        }
      })

      return { previousData }
    },
    onError: (error: Error, id, context) => {
      // Rollback on error
      if (context?.previousData) {
        queryClient.setQueryData(['paymentMethods'], context.previousData)
      }
      toast.error(error.message)
    },
    onSettled: () => {
      // Always refetch to ensure data is in sync
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
    },
  })

  const initMutation = useMutation({
    mutationFn: initBuiltinPaymentMethods,
    onSuccess: () => {
      toast.success(t.admin.pmBuiltinInitialized)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  const testMutation = useMutation({
    mutationFn: ({ script, config }: { script: string; config: Record<string, any> }) =>
      testPaymentScript(script, config),
    onSuccess: (data: any) => {
      setTestResult(data?.data?.html || JSON.stringify(data?.data, null, 2))
    },
    onError: (error: Error) => {
      setTestResult(`Error: ${error.message}`)
    },
  })

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      type: 'custom',
      icon: 'CreditCard',
      script: '',
      config: '{}',
      poll_interval: 30,
    })
  }

  const openEdit = (method: PaymentMethod) => {
    setEditingMethod(method)
    setFormData({
      name: method.name,
      description: method.description || '',
      type: 'custom',
      icon: method.icon || 'CreditCard',
      script: method.script || '',
      config: method.config || '{}',
      poll_interval: method.poll_interval || 30,
    })
  }

  const handleSubmit = () => {
    // Flush any pending debounced config updates and get the latest value
    const latestConfig = configFlushRef.current?.() ?? formData.config

    const data = {
      name: formData.name,
      description: formData.description,
      type: 'custom' as const,
      icon: formData.icon,
      script: formData.script,
      config: latestConfig,
      poll_interval: formData.poll_interval,
    }

    if (editingMethod) {
      updateMutation.mutate({ id: editingMethod.id, data })
    } else {
      createMutation.mutate(data)
    }
  }

  const handleTest = () => {
    try {
      const config = JSON.parse(formData.config || '{}')
      testMutation.mutate({ script: formData.script, config })
    } catch (e) {
      toast.error(t.admin.pmInvalidConfigJson)
    }
  }

  const getIcon = (iconName: string) => {
    const Icon = iconMap[iconName] || CreditCard
    return <Icon className="h-5 w-5" />
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-3xl font-bold">{t.admin.pmTitle}</h1>
        <div className="text-center py-8">{t.common.loading}</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.pmTitle}</h1>
          <p className="text-muted-foreground mt-1">
            {t.admin.pmSubtitle}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => initMutation.mutate()}>
            <RefreshCw className="h-4 w-4 mr-2" />
            {t.admin.pmInitBuiltin}
          </Button>
          <Button onClick={() => { resetForm(); setIsCreateOpen(true) }}>
            <Plus className="h-4 w-4 mr-2" />
            {t.admin.pmAdd}
          </Button>
        </div>
      </div>

      {/* 付款方式列表 */}
      <div className="grid gap-4">
        {methods.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              {t.admin.pmNoMethods}
            </CardContent>
          </Card>
        ) : (
          methods.map((method: PaymentMethod) => (
            <Card key={method.id} className={!method.enabled ? 'opacity-60' : ''}>
              <CardContent className="p-4">
                <div className="flex items-center gap-4">
                  <div className="cursor-move text-muted-foreground">
                    <GripVertical className="h-5 w-5" />
                  </div>
                  <div className="p-2 rounded-lg bg-muted">
                    {getIcon(method.icon || 'CreditCard')}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold">{method.name}</h3>
                      <Badge variant="default">JS</Badge>
                    </div>
                    <p className="text-sm text-muted-foreground truncate">{method.description}</p>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="flex items-center gap-2">
                      <Label htmlFor={`enabled-${method.id}`} className="text-sm">
                        {t.admin.enabled}
                      </Label>
                      <Switch
                        id={`enabled-${method.id}`}
                        checked={method.enabled}
                        onCheckedChange={() => toggleMutation.mutate(method.id)}
                      />
                    </div>
                    <Button variant="outline" size="sm" onClick={() => openEdit(method)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button variant="destructive" size="sm" onClick={() => setDeleteId(method.id)}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>

      {/* 创建/编辑对话框 */}
      <Dialog open={isCreateOpen || !!editingMethod} onOpenChange={(open) => {
        if (!open) {
          setIsCreateOpen(false)
          setEditingMethod(null)
          setTestResult(null)
        }
      }}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editingMethod ? t.admin.pmEdit : t.admin.pmAdd}
            </DialogTitle>
            <DialogDescription>
              {t.admin.pmDialogDesc}
            </DialogDescription>
          </DialogHeader>

          <Tabs defaultValue="basic" className="w-full">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="basic">
                <Settings className="h-4 w-4 mr-2" />
                {t.admin.pmTabBasic}
              </TabsTrigger>
              <TabsTrigger value="config">
                <CreditCard className="h-4 w-4 mr-2" />
                {t.admin.pmTabConfig}
              </TabsTrigger>
              <TabsTrigger value="script">
                <Code className="h-4 w-4 mr-2" />
                {t.admin.pmTabScript}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="basic" className="space-y-4 mt-4">
              <div className="space-y-2">
                <Label>{t.admin.pmName}</Label>
                <Input
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder={t.admin.pmNamePlaceholder}
                />
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pmDescription}</Label>
                <Textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder={t.admin.pmDescPlaceholder}
                  rows={3}
                />
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pmIcon}</Label>
                <Select
                  value={formData.icon}
                  onValueChange={(v) => setFormData({ ...formData, icon: v })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {Object.keys(iconMap).map((icon) => (
                      <SelectItem key={icon} value={icon}>
                        <div className="flex items-center gap-2">
                          {getIcon(icon)}
                          <span>{icon}</span>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>{t.admin.pmPollInterval}</Label>
                <Input
                  type="number"
                  min={5}
                  max={600}
                  value={formData.poll_interval}
                  onChange={(e) => setFormData({ ...formData, poll_interval: parseInt(e.target.value) || 30 })}
                  placeholder="30"
                />
                <p className="text-xs text-muted-foreground">
                  {t.admin.pmPollIntervalHint}
                </p>
              </div>
            </TabsContent>

            <TabsContent value="config" className="space-y-4 mt-4">
              <ConfigEditor
                key={editingMethod?.id ?? 'new'}
                value={formData.config}
                onChange={(v) => setFormData({ ...formData, config: v })}
                flushRef={configFlushRef}
                t={t}
              />
            </TabsContent>

            <TabsContent value="script" className="space-y-4 mt-4">
              <div className="space-y-2">
                <Label>{t.admin.pmJsScript}</Label>
                <Textarea
                  value={formData.script}
                  onChange={(e) => setFormData({ ...formData, script: e.target.value })}
                  placeholder={`// 生成付款卡片HTML
function onGeneratePaymentCard(order, config) {
  return {
    html: '<div>自定义付款信息</div>',
    title: '付款方式名称',
    cache_ttl: 300  // 缓存5分钟 (0=不缓存, -1=永久, >0=秒数)
  }
}`}
                  rows={15}
                  className="font-mono text-sm"
                />
                <div className="flex gap-2">
                  <Button variant="outline" onClick={handleTest} disabled={testMutation.isPending}>
                    <Play className="h-4 w-4 mr-2" />
                    {t.admin.pmTestScript}
                  </Button>
                </div>
              </div>
              {testResult && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">{t.admin.pmTestResult}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <SandboxedHtmlFrame
                      html={testResult}
                      title={t.admin.pmTestResult}
                      className="max-h-64"
                      locale={locale}
                    />
                  </CardContent>
                </Card>
              )}
              <Card className="bg-muted/50">
                <CardHeader>
                  <CardTitle className="text-sm">{t.admin.pmApiRef}</CardTitle>
                  <CardDescription className="text-xs">
                    {t.admin.pmApiRefDesc}
                  </CardDescription>
                </CardHeader>
                <CardContent className="text-xs space-y-3">
                  <div>
                    <p className="font-semibold mb-1">{t.admin.pmRequiredCallbacks}</p>
                    <p><code>onGeneratePaymentCard(order, config)</code> - {t.admin.pmGenerateCardHtml}</p>
                    <p className="text-muted-foreground ml-4">
                      {t.admin.pmReturns}
                      <code>{`{html, title?, description?, data?, cache_ttl?}`}</code>
                    </p>
                    <p className="text-muted-foreground ml-4">
                      <code>cache_ttl</code>: {t.admin.pmCacheTtlDesc}
                    </p>
                    <p><code>onCheckPaymentStatus(order, config)</code> - {t.admin.pmCheckStatus}</p>
                    <p className="text-muted-foreground ml-4">
                      {t.admin.pmReturns}
                      <code>{`{paid: boolean, message?, transaction_id?, data?}`}</code>
                    </p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.storage <span className="font-normal text-muted-foreground">({t.admin.pmLocalStorage})</span></p>
                    <p><code>get(key)</code> / <code>set(key, value)</code> / <code>delete(key)</code></p>
                    <p><code>list()</code> / <code>clear()</code></p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.order</p>
                    <p><code>get()</code> - {t.admin.pmGetOrder}</p>
                    <p><code>getItems()</code> - {t.admin.pmGetOrderItems}</p>
                    <p><code>getUser()</code> - {t.admin.pmGetOrderUser}</p>
                    <p><code>updatePaymentData(data)</code> - {t.admin.pmUpdatePaymentData}</p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.config</p>
                    <p><code>get(key?, defaultValue?)</code> - {t.admin.pmGetConfigValue}</p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.utils</p>
                    <p><code>formatPrice(amount, currency)</code> - {t.admin.pmFormatPrice}</p>
                    <p><code>formatDate(date, format?)</code> - {t.admin.pmFormatDate}</p>
                    <p><code>generateId()</code> - {t.admin.pmGenerateUuid}</p>
                    <p><code>md5(data)</code> / <code>base64Encode(data)</code> / <code>base64Decode(data)</code></p>
                    <p><code>jsonEncode(data)</code> / <code>jsonDecode(data)</code></p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.http</p>
                    <p><code>get(url, headers?)</code> - {t.admin.pmGetRequest}</p>
                    <p><code>post(url, body?, headers?)</code> - {t.admin.pmPostRequest}</p>
                    <p><code>request(options)</code> - {t.admin.pmGeneralRequest}</p>
                    <p className="text-muted-foreground">{t.admin.pmHttpReturns}</p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.system</p>
                    <p><code>getTimestamp()</code> - {t.admin.pmGetTimestamp}</p>
                    <p><code>getPaymentMethodInfo()</code> - {t.admin.pmGetMethodInfo}</p>
                  </div>
                  <div className="border-t pt-3 mt-3">
                    <p className="font-semibold mb-1">{t.admin.pmThemeAdaptation}</p>
                    <p className="text-muted-foreground">
                      {t.admin.pmThemeAdaptationDesc}
                    </p>
                    <p className="mt-1">
                      <code className="text-green-600 dark:text-green-400">bg-muted</code>, <code className="text-green-600 dark:text-green-400">text-muted-foreground</code>, <code className="text-green-600 dark:text-green-400">text-primary</code>, <code className="text-green-600 dark:text-green-400">border-border</code>
                    </p>
                    <p className="mt-1">
                      {t.admin.pmDarkModeStyles}
                      <code className="text-blue-600 dark:text-blue-400">dark:bg-gray-800</code>, <code className="text-blue-600 dark:text-blue-400">dark:text-gray-100</code>
                    </p>
                  </div>
                  <div className="border-t pt-3 mt-3">
                    <p className="font-semibold mb-1">{t.admin.pmMultiLanguage}</p>
                    <p className="text-muted-foreground">
                      {t.admin.pmMultiLanguageDesc}
                    </p>
                    <p className="mt-1">
                      <code className="text-purple-600 dark:text-purple-400">&lt;span class="lang-zh"&gt;中文&lt;/span&gt;</code>
                      <code className="text-purple-600 dark:text-purple-400 ml-2">&lt;span class="lang-en"&gt;English&lt;/span&gt;</code>
                    </p>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button variant="outline" onClick={() => {
              setIsCreateOpen(false)
              setEditingMethod(null)
              setTestResult(null)
            }}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
              {editingMethod ? t.common.save : t.common.create}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 删除确认 */}
      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.admin.pmDeleteDesc}
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
