'use client'

import { useState, useEffect } from 'react'
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

export default function PaymentMethodsPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminPaymentMethods)
  const queryClient = useQueryClient()
  const [editingMethod, setEditingMethod] = useState<PaymentMethod | null>(null)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [testResult, setTestResult] = useState<string | null>(null)

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
      toast.success(locale === 'zh' ? '创建成功' : 'Created successfully')
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
      toast.success(locale === 'zh' ? '更新成功' : 'Updated successfully')
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
      toast.success(locale === 'zh' ? '删除成功' : 'Deleted successfully')
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
      toast.success(locale === 'zh' ? '内置付款方式已初始化' : 'Built-in methods initialized')
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
    const data = {
      name: formData.name,
      description: formData.description,
      type: 'custom' as const,
      icon: formData.icon,
      script: formData.script,
      config: formData.config,
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
      toast.error(locale === 'zh' ? '配置JSON格式错误' : 'Invalid config JSON')
    }
  }

  const getIcon = (iconName: string) => {
    const Icon = iconMap[iconName] || CreditCard
    return <Icon className="h-5 w-5" />
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-3xl font-bold">{locale === 'zh' ? '付款方式管理' : 'Payment Methods'}</h1>
        <div className="text-center py-8">{locale === 'zh' ? '加载中...' : 'Loading...'}</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{locale === 'zh' ? '付款方式管理' : 'Payment Methods'}</h1>
          <p className="text-muted-foreground mt-1">
            {locale === 'zh' ? '管理系统付款方式，所有付款方式均使用JS脚本扩展' : 'Manage payment methods with JS script extensions'}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => initMutation.mutate()}>
            <RefreshCw className="h-4 w-4 mr-2" />
            {locale === 'zh' ? '初始化内置' : 'Init Built-in'}
          </Button>
          <Button onClick={() => { resetForm(); setIsCreateOpen(true) }}>
            <Plus className="h-4 w-4 mr-2" />
            {locale === 'zh' ? '添加付款方式' : 'Add Method'}
          </Button>
        </div>
      </div>

      {/* 付款方式列表 */}
      <div className="grid gap-4">
        {methods.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              {locale === 'zh' ? '暂无付款方式，点击"初始化内置"添加默认付款方式' : 'No payment methods. Click "Init Built-in" to add defaults'}
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
                        {locale === 'zh' ? '启用' : 'Enabled'}
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
              {editingMethod
                ? (locale === 'zh' ? '编辑付款方式' : 'Edit Payment Method')
                : (locale === 'zh' ? '添加付款方式' : 'Add Payment Method')}
            </DialogTitle>
            <DialogDescription>
              {locale === 'zh' ? '配置付款方式的基本信息和扩展脚本' : 'Configure payment method details and extension script'}
            </DialogDescription>
          </DialogHeader>

          <Tabs defaultValue="basic" className="w-full">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="basic">
                <Settings className="h-4 w-4 mr-2" />
                {locale === 'zh' ? '基本信息' : 'Basic'}
              </TabsTrigger>
              <TabsTrigger value="config">
                <CreditCard className="h-4 w-4 mr-2" />
                {locale === 'zh' ? '配置' : 'Config'}
              </TabsTrigger>
              <TabsTrigger value="script">
                <Code className="h-4 w-4 mr-2" />
                {locale === 'zh' ? 'JS脚本' : 'JS Script'}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="basic" className="space-y-4 mt-4">
              <div className="space-y-2">
                <Label>{locale === 'zh' ? '名称' : 'Name'}</Label>
                <Input
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder={locale === 'zh' ? '付款方式名称' : 'Payment method name'}
                />
              </div>
              <div className="space-y-2">
                <Label>{locale === 'zh' ? '描述' : 'Description'}</Label>
                <Textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder={locale === 'zh' ? '付款方式描述' : 'Payment method description'}
                  rows={3}
                />
              </div>
              <div className="space-y-2">
                <Label>{locale === 'zh' ? '图标' : 'Icon'}</Label>
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
                <Label>{locale === 'zh' ? '轮询检查间隔 (秒)' : 'Poll Interval (seconds)'}</Label>
                <Input
                  type="number"
                  min={5}
                  max={600}
                  value={formData.poll_interval}
                  onChange={(e) => setFormData({ ...formData, poll_interval: parseInt(e.target.value) || 30 })}
                  placeholder="30"
                />
                <p className="text-xs text-muted-foreground">
                  {locale === 'zh'
                    ? '自动检查付款状态的时间间隔，建议 30-60 秒。区块链付款建议 30 秒，银行转账建议 60 秒或更长。'
                    : 'Interval for checking payment status automatically. 30-60 seconds recommended.'}
                </p>
              </div>
            </TabsContent>

            <TabsContent value="config" className="space-y-4 mt-4">
              <div className="space-y-2">
                <Label>{locale === 'zh' ? '配置 (JSON格式)' : 'Config (JSON)'}</Label>
                <Textarea
                  value={formData.config}
                  onChange={(e) => setFormData({ ...formData, config: e.target.value })}
                  placeholder='{"bank_name": "xxx", "account_number": "xxx"}'
                  rows={10}
                  className="font-mono text-sm"
                />
                <p className="text-xs text-muted-foreground">
                  {locale === 'zh'
                    ? '配置项如：银行名称、账号、收款码URL、钱包地址等'
                    : 'Config fields like: bank name, account number, QR code URL, wallet address, etc.'}
                </p>
              </div>
            </TabsContent>

            <TabsContent value="script" className="space-y-4 mt-4">
              <div className="space-y-2">
                <Label>{locale === 'zh' ? 'JavaScript 脚本' : 'JavaScript Script'}</Label>
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
                    {locale === 'zh' ? '测试脚本' : 'Test Script'}
                  </Button>
                </div>
              </div>
              {testResult && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm">{locale === 'zh' ? '测试结果' : 'Test Result'}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="bg-muted p-4 rounded-lg overflow-auto max-h-48">
                      <SandboxedHtmlFrame
                        html={testResult}
                        title={locale === 'zh' ? '测试结果' : 'Test Result'}
                        className="h-48"
                      />
                    </div>
                  </CardContent>
                </Card>
              )}
              <Card className="bg-muted/50">
                <CardHeader>
                  <CardTitle className="text-sm">{locale === 'zh' ? 'API 参考' : 'API Reference'}</CardTitle>
                  <CardDescription className="text-xs">
                    {locale === 'zh' ? '完整文档见 docs/PAYMENT_JS_API.md' : 'Full docs: docs/PAYMENT_JS_API.md'}
                  </CardDescription>
                </CardHeader>
                <CardContent className="text-xs space-y-3">
                  <div>
                    <p className="font-semibold mb-1">{locale === 'zh' ? '必须实现的回调函数' : 'Required Callbacks'}</p>
                    <p><code>onGeneratePaymentCard(order, config)</code> - {locale === 'zh' ? '生成付款卡片HTML' : 'Generate payment card HTML'}</p>
                    <p className="text-muted-foreground ml-4">
                      {locale === 'zh' ? '返回: ' : 'Returns: '}
                      <code>{`{html, title?, description?, data?, cache_ttl?}`}</code>
                    </p>
                    <p className="text-muted-foreground ml-4">
                      <code>cache_ttl</code>: {locale === 'zh' ? '缓存有效期(秒)。0=不缓存，-1=永久缓存，>0=指定秒数后过期' : 'Cache TTL (seconds). 0=no cache, -1=permanent, >0=expire after seconds'}
                    </p>
                    <p><code>onCheckPaymentStatus(order, config)</code> - {locale === 'zh' ? '检查付款状态' : 'Check payment status'}</p>
                    <p className="text-muted-foreground ml-4">
                      {locale === 'zh' ? '返回: ' : 'Returns: '}
                      <code>{`{paid: boolean, message?, transaction_id?, data?}`}</code>
                    </p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.storage <span className="font-normal text-muted-foreground">({locale === 'zh' ? '本地持久化存储' : 'local persistent storage'})</span></p>
                    <p><code>get(key)</code> / <code>set(key, value)</code> / <code>delete(key)</code></p>
                    <p><code>list()</code> / <code>clear()</code></p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.order</p>
                    <p><code>get()</code> - {locale === 'zh' ? '获取当前订单 (id, order_no, status, total_amount, currency)' : 'Get current order'}</p>
                    <p><code>getItems()</code> - {locale === 'zh' ? '获取订单商品列表' : 'Get order items'}</p>
                    <p><code>getUser()</code> - {locale === 'zh' ? '获取订单用户信息' : 'Get order user'}</p>
                    <p><code>updatePaymentData(data)</code> - {locale === 'zh' ? '更新付款数据' : 'Update payment data'}</p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.config</p>
                    <p><code>get(key?, defaultValue?)</code> - {locale === 'zh' ? '获取配置值' : 'Get config value'}</p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.utils</p>
                    <p><code>formatPrice(amount, currency)</code> - {locale === 'zh' ? '格式化价格' : 'Format price'}</p>
                    <p><code>formatDate(date, format?)</code> - {locale === 'zh' ? '格式化日期' : 'Format date'}</p>
                    <p><code>generateId()</code> - {locale === 'zh' ? '生成UUID' : 'Generate UUID'}</p>
                    <p><code>md5(data)</code> / <code>base64Encode(data)</code> / <code>base64Decode(data)</code></p>
                    <p><code>jsonEncode(data)</code> / <code>jsonDecode(data)</code></p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.http</p>
                    <p><code>get(url, headers?)</code> - {locale === 'zh' ? 'GET请求' : 'GET request'}</p>
                    <p><code>post(url, body?, headers?)</code> - {locale === 'zh' ? 'POST请求' : 'POST request'}</p>
                    <p><code>request(options)</code> - {locale === 'zh' ? '通用HTTP请求' : 'General HTTP request'}</p>
                    <p className="text-muted-foreground">{locale === 'zh' ? '返回: {status, headers, body, data, error}' : 'Returns: {status, headers, body, data, error}'}</p>
                  </div>
                  <div>
                    <p className="font-semibold mb-1">AuraLogic.system</p>
                    <p><code>getTimestamp()</code> - {locale === 'zh' ? '获取Unix时间戳' : 'Get Unix timestamp'}</p>
                    <p><code>getPaymentMethodInfo()</code> - {locale === 'zh' ? '获取付款方式信息' : 'Get payment method info'}</p>
                  </div>
                  <div className="border-t pt-3 mt-3">
                    <p className="font-semibold mb-1">{locale === 'zh' ? '主题适配提示' : 'Theme Adaptation'}</p>
                    <p className="text-muted-foreground">
                      {locale === 'zh'
                        ? '生成的HTML会在支持亮色/暗色主题的前端渲染。请使用主题感知的CSS类：'
                        : 'Generated HTML renders in a themed frontend. Use theme-aware CSS classes:'}
                    </p>
                    <p className="mt-1">
                      <code className="text-green-600 dark:text-green-400">bg-muted</code>, <code className="text-green-600 dark:text-green-400">text-muted-foreground</code>, <code className="text-green-600 dark:text-green-400">text-primary</code>, <code className="text-green-600 dark:text-green-400">border-border</code>
                    </p>
                    <p className="mt-1">
                      {locale === 'zh' ? '暗色模式特定样式: ' : 'Dark mode styles: '}
                      <code className="text-blue-600 dark:text-blue-400">dark:bg-gray-800</code>, <code className="text-blue-600 dark:text-blue-400">dark:text-gray-100</code>
                    </p>
                  </div>
                  <div className="border-t pt-3 mt-3">
                    <p className="font-semibold mb-1">{locale === 'zh' ? '多语言支持' : 'Multi-language'}</p>
                    <p className="text-muted-foreground">
                      {locale === 'zh'
                        ? '使用语言类自动切换中英文内容：'
                        : 'Use language classes for auto-switching content:'}
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
              {locale === 'zh' ? '取消' : 'Cancel'}
            </Button>
            <Button onClick={handleSubmit} disabled={createMutation.isPending || updateMutation.isPending}>
              {editingMethod
                ? (locale === 'zh' ? '保存' : 'Save')
                : (locale === 'zh' ? '创建' : 'Create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 删除确认 */}
      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{locale === 'zh' ? '确认删除' : 'Confirm Delete'}</AlertDialogTitle>
            <AlertDialogDescription>
              {locale === 'zh' ? '确定要删除这个付款方式吗？此操作无法撤销。' : 'Are you sure you want to delete this payment method? This action cannot be undone.'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{locale === 'zh' ? '取消' : 'Cancel'}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
              className="bg-red-600 hover:bg-red-700"
            >
              {locale === 'zh' ? '删除' : 'Delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
