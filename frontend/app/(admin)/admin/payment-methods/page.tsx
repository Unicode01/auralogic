'use client'

import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  AdminPaymentMethodMarketPreviewRequest,
  getPaymentMethods,
  createPaymentMethod,
  updatePaymentMethod,
  deletePaymentMethod,
  togglePaymentMethodEnabled,
  reorderPaymentMethods,
  testPaymentScript,
  initBuiltinPaymentMethods,
  PaymentMethod,
  PaymentMethodMarketPreview,
  PaymentMethodPackagePreview,
  previewPaymentMethodMarketPackage,
  previewPaymentMethodPackage,
  importPaymentMethodPackageFromMarket,
  uploadPaymentMethodPackage,
} from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import CodeMirror from '@uiw/react-codemirror'
import { javascript } from '@codemirror/lang-javascript'
import { useTheme } from '@/contexts/theme-context'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { SandboxedHtmlFrame } from '@/components/ui/sandboxed-html-frame'
import { renderToastMessage } from '@/components/ui/toast-message'
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
  FileUp,
  Loader2,
  Package,
} from 'lucide-react'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { usePluginBootstrapQuery } from '@/lib/plugin-bootstrap-query'
import {
  buildAdminMarketPluginPageHref,
  findAdminMarketPluginBasePath,
} from '@/lib/plugin-market-route'
import { PaymentMethodWebhookPanel } from '@/components/admin/payment-method-webhook-panel'
import { PluginJSONObjectEditor } from '@/components/admin/plugins/plugin-json-object-editor'
import { PluginJSONSchemaEditor } from '@/components/admin/plugins/plugin-json-schema-editor'
import { ConfigEditor } from '@/components/admin/config-editor'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import type { PluginJSONSchema } from '@/components/admin/plugins/types'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'
import {
  findMissingRequiredSchemaFields,
  formatHumanReadableJSONText,
  manifestString,
  parseJSONObjectText,
  parseManifestObjectSchema,
  tryFormatTextareaJSON,
} from '@/lib/package-manifest-schema'

const iconMap: Record<string, any> = {
  CreditCard,
  Building2,
  Wallet,
  MessageCircle,
  Bitcoin,
  Code,
  Coins,
}
type PaymentPackageFormState = {
  target_id: string
  name: string
  description: string
  icon: string
  version: string
  entry: string
  config: string
  poll_interval: number
}

function createDefaultPackageForm(): PaymentPackageFormState {
  return {
    target_id: 'new',
    name: '',
    description: '',
    icon: 'CreditCard',
    version: '',
    entry: '',
    config: '{}',
    poll_interval: 30,
  }
}

function formatScriptBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function buildAdminPaymentMethodSummary(method: PaymentMethod) {
  return {
    id: method.id,
    name: method.name,
    description: method.description,
    type: method.type,
    enabled: method.enabled,
    icon: method.icon,
    version: method.version,
    package_name: method.package_name,
    package_entry: method.package_entry,
    package_checksum: method.package_checksum,
    sort_order: method.sort_order,
    poll_interval: method.poll_interval,
    script_length: method.script?.length || 0,
    config_length: method.config?.length || 0,
    created_at: method.created_at,
    updated_at: method.updated_at,
  }
}

function normalizeUniqueTextList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const items: string[] = []
  value.forEach((item) => {
    if (typeof item !== 'string') return
    const normalized = item.trim()
    if (!normalized) return
    const key = normalized.toLowerCase()
    if (seen.has(key)) return
    seen.add(key)
    items.push(normalized)
  })
  return items
}

function resolvePaymentPackageImportModeLabel(
  locale: string,
  options: { installed?: boolean; updateAvailable?: boolean }
): string {
  if (!options.installed) {
    return locale === 'zh' ? '新建导入' : 'Fresh import'
  }
  if (options.updateAvailable) {
    return locale === 'zh' ? '更新现有方式' : 'Update existing method'
  }
  return locale === 'zh' ? '覆盖当前版本' : 'Reapply current version'
}

function normalizeMarketQueryKinds(value: string): string[] {
  return String(value || '')
    .split(/[;,]/)
    .map((item) => item.trim().toLowerCase())
    .filter((item, index, source) => !!item && source.indexOf(item) === index)
}

function parseMarketPaymentImportRequest(
  searchParams: ReturnType<typeof useSearchParams>
): AdminPaymentMethodMarketPreviewRequest | null {
  const enabled = String(searchParams?.get('market_import') || '')
    .trim()
    .toLowerCase()
  if (!enabled || !['1', 'true', 'yes'].includes(enabled)) {
    return null
  }
  const sourceID = String(searchParams?.get('market_source_id') || '').trim()
  const baseURL = String(searchParams?.get('market_source_base_url') || '').trim()
  const kind = String(searchParams?.get('market_kind') || '')
    .trim()
    .toLowerCase()
  const name = String(searchParams?.get('market_name') || '').trim()
  const version = String(searchParams?.get('market_version') || '').trim()
  if (!baseURL || kind !== 'payment_package' || !name || !version) {
    return null
  }
  return {
    source: {
      source_id: sourceID || 'market',
      name: String(searchParams?.get('market_source_name') || '').trim() || undefined,
      base_url: baseURL,
      public_key: String(searchParams?.get('market_source_public_key') || '').trim() || undefined,
      default_channel: String(searchParams?.get('market_source_channel') || '').trim() || undefined,
      allowed_kinds: normalizeMarketQueryKinds(
        String(searchParams?.get('market_source_allowed_kinds') || kind)
      ),
      enabled: true,
    },
    kind,
    name,
    version,
  }
}

function buildMarketPaymentImportRequestKey(
  request: AdminPaymentMethodMarketPreviewRequest | null,
  targetID: string
): string {
  if (!request) return ''
  return [
    request.source.source_id,
    request.source.base_url,
    request.kind,
    request.name,
    request.version,
    targetID || 'new',
  ].join('|')
}

export default function PaymentMethodsPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminPaymentMethods)
  const { resolvedTheme } = useTheme()
  const router = useRouter()
  const queryClient = useQueryClient()
  const searchParams = useSearchParams()
  const [editingMethod, setEditingMethod] = useState<PaymentMethod | null>(null)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isImportOpen, setIsImportOpen] = useState(false)
  const [deleteId, setDeleteId] = useState<number | null>(null)
  const [testResult, setTestResult] = useState<string | null>(null)
  const configFlushRef = useRef<(() => string | null) | null>(null)
  const packageFileInputRef = useRef<HTMLInputElement | null>(null)
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [overIndex, setOverIndex] = useState<number | null>(null)
  const [packageInputKey, setPackageInputKey] = useState(0)
  const [packageFile, setPackageFile] = useState<File | null>(null)
  const [packagePreview, setPackagePreview] = useState<PaymentMethodPackagePreview | null>(null)
  const [packageForm, setPackageForm] = useState<PaymentPackageFormState>(createDefaultPackageForm)
  const [marketImportContext, setMarketImportContext] = useState<PaymentMethodMarketPreview | null>(
    null
  )
  const [marketImportPreviewKey, setMarketImportPreviewKey] = useState('')
  const resolveAdminError = (error: unknown, fallback: string) =>
    resolveApiErrorMessage(error, t, fallback)
  const showAdminErrorToast = (error: unknown, fallback: string) => {
    const message = resolveAdminError(error, fallback)
    toast.error(renderToastMessage(message))
  }
  const adminBootstrapQuery = usePluginBootstrapQuery({
    scope: 'admin',
    path: '/admin',
    staleTime: 5 * 60 * 1000,
  })
  const marketImportRequest = useMemo(
    () => parseMarketPaymentImportRequest(searchParams),
    [searchParams]
  )
  const marketImportRequestKey = useMemo(
    () => buildMarketPaymentImportRequestKey(marketImportRequest, packageForm.target_id),
    [marketImportRequest, packageForm.target_id]
  )
  const isMarketImportMode = !!marketImportRequest
  const marketPluginImportHref = useMemo(() => {
    return buildAdminMarketPluginPageHref(findAdminMarketPluginBasePath(adminBootstrapQuery.data), {
      kind: 'payment_package',
    })
  }, [adminBootstrapQuery.data])

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
  const webhookExampleHook = 'payment.notify'
  const webhookExampleURL = editingMethod
    ? `/api/payment-methods/${editingMethod.id}/webhooks/${webhookExampleHook}`
    : `/api/payment-methods/{id}/webhooks/${webhookExampleHook}`
  const webhookSectionTitle = locale === 'zh' ? 'Webhook 回调' : 'Webhook callbacks'
  const webhookSectionDesc =
    locale === 'zh'
      ? '公开回调入口。脚本可实现 onWebhook(hook, config) 并通过 AuraLogic.webhook 读取请求原文。'
      : 'Public callback entry. Implement onWebhook(hook, config) and read the raw request through AuraLogic.webhook.'
  const webhookReturnsText =
    locale === 'zh'
      ? '可返回 { paid?, order_id?/order_no?, transaction_id?, message?, data?, queue_polling?, ack_status?, ack_body?, ack_headers? }'
      : 'May return { paid?, order_id?/order_no?, transaction_id?, message?, data?, queue_polling?, ack_status?, ack_body?, ack_headers? }'
  const webhookHintText =
    locale === 'zh'
      ? '建议在生成远端支付单时调用 AuraLogic.system.getWebhookUrl("payment.notify") 作为回调地址。'
      : 'Use AuraLogic.system.getWebhookUrl("payment.notify") as the callback URL when creating upstream payment sessions.'

  const { data, isLoading } = useQuery({
    queryKey: ['paymentMethods'],
    queryFn: () => getPaymentMethods(),
    staleTime: 0, // 数据立即过期，每次都重新获取
    refetchOnMount: 'always', // 组件挂载时总是重新获取
  })

  const methods = data?.data?.items || []
  const deleteMethod =
    deleteId !== null
      ? methods.find((method: PaymentMethod) => method.id === deleteId) || null
      : null
  const packageConfigSchema = useMemo<PluginJSONSchema | null>(
    () => parseManifestObjectSchema(packagePreview?.manifest?.config_schema, locale),
    [locale, packagePreview?.manifest?.config_schema]
  )
  const packageConfigObject = useMemo(
    () => parseJSONObjectText(packageForm.config),
    [packageForm.config]
  )
  const packageConfigValid = packageConfigObject !== null
  const packageMissingRequiredFields = useMemo(
    () => findMissingRequiredSchemaFields(packageConfigSchema, packageConfigObject),
    [packageConfigObject, packageConfigSchema]
  )
  const packageManifestName = manifestString(packagePreview?.manifest, 'display_name', locale)
  const packageManifestDescription = manifestString(packagePreview?.manifest, 'description', locale)
  const packageWebhookCount = packagePreview?.manifest?.webhooks?.length || 0
  const packageImportTargetPaymentMethodId =
    packageForm.target_id !== 'new' ? packageForm.target_id : null

  const resetPackageImportState = (options?: { keepMarketContext?: boolean }) => {
    setPackageFile(null)
    setPackagePreview(null)
    setPackageForm(createDefaultPackageForm())
    setPackageInputKey((prev) => prev + 1)
    if (!options?.keepMarketContext) {
      setMarketImportContext(null)
      setMarketImportPreviewKey('')
    }
  }

  const exitMarketImportMode = () => {
    if (!isMarketImportMode) {
      return
    }
    router.replace('/admin/payment-methods', { scroll: false })
  }

  const closeImportDialog = () => {
    setIsImportOpen(false)
    if (isMarketImportMode) {
      setPackageFile(null)
      setPackagePreview(null)
      setPackageInputKey((prev) => prev + 1)
      setMarketImportContext(null)
      exitMarketImportMode()
      return
    }
    resetPackageImportState()
  }

  const createMutation = useMutation({
    mutationFn: createPaymentMethod,
    onSuccess: () => {
      toast.success(t.admin.pmCreatedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      setIsCreateOpen(false)
      resetForm()
    },
    onError: (error: any) => {
      showAdminErrorToast(error, t.admin.operationFailed)
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
    onError: (error: any) => {
      showAdminErrorToast(error, t.admin.operationFailed)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deletePaymentMethod,
    onSuccess: () => {
      toast.success(t.admin.pmDeletedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      setDeleteId(null)
    },
    onError: (error: any) => {
      showAdminErrorToast(error, t.admin.operationFailed)
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
            ),
          },
        }
      })

      return { previousData }
    },
    onError: (error: any, id, context) => {
      // Rollback on error
      if (context?.previousData) {
        queryClient.setQueryData(['paymentMethods'], context.previousData)
      }
      showAdminErrorToast(error, t.admin.operationFailed)
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
    onError: (error: any) => {
      showAdminErrorToast(error, t.admin.operationFailed)
    },
  })

  const reorderMutation = useMutation({
    mutationFn: reorderPaymentMethods,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
    },
    onError: (error: any) => {
      showAdminErrorToast(error, t.admin.operationFailed)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
    },
  })

  const testMutation = useMutation({
    mutationFn: ({ script, config }: { script: string; config: Record<string, any> }) =>
      testPaymentScript(script, config),
    onSuccess: (data: any) => {
      setTestResult(data?.data?.html || JSON.stringify(data?.data, null, 2))
    },
    onError: (error: unknown) => {
      setTestResult(resolveAdminError(error, t.admin.operationFailed))
    },
  })

  const previewPackageMutation = useMutation({
    mutationFn: (input: { file: File; targetId?: string }) => {
      const payload = new FormData()
      payload.append('file', input.file)
      if (input.targetId && input.targetId !== 'new') {
        payload.append('payment_method_id', input.targetId)
      }
      return previewPaymentMethodPackage(payload)
    },
    onSuccess: (result: any) => {
      const preview = (result?.data || null) as PaymentMethodPackagePreview | null
      if (!preview?.resolved) {
        toast.error(t.admin.operationFailed)
        return
      }
      setPackagePreview(preview)
      setPackageForm((prev) => ({
        ...prev,
        name:
          manifestString(preview.manifest, 'display_name', locale) || preview.resolved.name || '',
        description:
          manifestString(preview.manifest, 'description', locale) ||
          preview.resolved.description ||
          '',
        icon: preview.resolved.icon || 'CreditCard',
        version: preview.resolved.version || '',
        entry: preview.resolved.entry || '',
        config: preview.resolved.config || '{}',
        poll_interval: preview.resolved.poll_interval || 30,
      }))
      toast.success(t.admin.pmPackagePreviewReady)
    },
    onError: (error: unknown) => {
      setPackagePreview(null)
      showAdminErrorToast(error, t.admin.operationFailed)
    },
  })

  const uploadPackageMutation = useMutation({
    mutationFn: (payload: FormData) => uploadPaymentMethodPackage(payload),
    onSuccess: () => {
      toast.success(t.admin.pmPackageImportedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      closeImportDialog()
    },
    onError: (error: unknown) => {
      showAdminErrorToast(error, t.admin.operationFailed)
    },
  })

  const previewMarketPackageMutation = useMutation({
    mutationFn: (request: AdminPaymentMethodMarketPreviewRequest) =>
      previewPaymentMethodMarketPackage(request),
    onSuccess: (result: any) => {
      const preview = (result?.data || null) as PaymentMethodMarketPreview | null
      if (!preview?.resolved) {
        toast.error(t.admin.operationFailed)
        return
      }
      setMarketImportContext(preview)
      setPackagePreview(preview)
      setPackageForm((prev) => ({
        ...prev,
        target_id: prev.target_id,
        name:
          manifestString(preview.manifest, 'display_name', locale) || preview.resolved.name || '',
        description:
          manifestString(preview.manifest, 'description', locale) ||
          preview.resolved.description ||
          '',
        icon: preview.resolved.icon || 'CreditCard',
        version: preview.resolved.version || '',
        entry: preview.resolved.entry || '',
        config: preview.resolved.config || '{}',
        poll_interval: preview.resolved.poll_interval || 30,
      }))
      setMarketImportPreviewKey(marketImportRequestKey)
      setIsImportOpen(true)
    },
    onError: (error: unknown) => {
      setPackagePreview(null)
      showAdminErrorToast(error, t.admin.operationFailed)
    },
  })

  const importMarketPackageMutation = useMutation({
    mutationFn: (payload: {
      request: AdminPaymentMethodMarketPreviewRequest
      payment_method_id?: number
      payment_name: string
      payment_description: string
      icon: string
      entry: string
      config: string
      poll_interval: number
    }) =>
      importPaymentMethodPackageFromMarket({
        source: payload.request.source,
        kind: payload.request.kind,
        name: payload.request.name,
        version: payload.request.version,
        payment_method_id: payload.payment_method_id,
        payment_name: payload.payment_name,
        payment_description: payload.payment_description,
        icon: payload.icon,
        entry: payload.entry,
        config: payload.config,
        poll_interval: payload.poll_interval,
      }),
    onSuccess: () => {
      toast.success(t.admin.pmPackageImportedSuccess)
      queryClient.invalidateQueries({ queryKey: ['paymentMethods'] })
      closeImportDialog()
    },
    onError: (error: unknown) => {
      showAdminErrorToast(error, t.admin.operationFailed)
    },
  })
  const importPackagePending =
    uploadPackageMutation.isPending || importMarketPackageMutation.isPending
  const marketCompatibility = (marketImportContext?.compatibility || null) as Record<
    string,
    any
  > | null
  const marketTargetState = (marketImportContext?.target_state || null) as Record<
    string,
    any
  > | null
  const marketWarnings = useMemo(
    () =>
      normalizeUniqueTextList([
        ...normalizeUniqueTextList(marketImportContext?.warnings),
        ...normalizeUniqueTextList(marketCompatibility?.warnings),
      ]),
    [marketCompatibility?.warnings, marketImportContext?.warnings]
  )
  const selectedPackageTargetMethod =
    methods.find((method: PaymentMethod) => String(method.id) === packageForm.target_id) || null
  const marketImportModeLabel = resolvePaymentPackageImportModeLabel(locale, {
    installed: marketTargetState?.installed === true,
    updateAvailable: marketTargetState?.update_available === true,
  })
  const adminPaymentMethodsPluginContext = {
    view: 'admin_payment_methods',
    summary: {
      total_methods: methods.length,
      enabled_count: methods.filter((method: PaymentMethod) => method.enabled).length,
      package_count: methods.filter((method: PaymentMethod) => Boolean(method.package_name)).length,
      market_warning_count: marketWarnings.length,
      is_market_import_mode: isMarketImportMode,
    },
    market: marketImportRequest
      ? {
          source_id: marketImportRequest.source.source_id,
          kind: marketImportRequest.kind,
          name: marketImportRequest.name,
          version: marketImportRequest.version,
          mode_label: marketImportModeLabel,
        }
      : undefined,
  }
  const adminPaymentMethodActionItems = methods.map((method: PaymentMethod, index: number) => ({
    key: String(method.id),
    slot: 'admin.payment_methods.row_actions',
    path: '/admin/payment-methods',
    hostContext: {
      view: 'admin_payment_method_row',
      method: buildAdminPaymentMethodSummary(method),
      row: {
        index: index + 1,
        selected: editingMethod?.id === method.id,
      },
      summary: adminPaymentMethodsPluginContext.summary,
    },
  }))
  const adminPaymentMethodActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/payment-methods',
    items: adminPaymentMethodActionItems,
    enabled: adminPaymentMethodActionItems.length > 0,
  })
  const adminPaymentMethodEditorPluginContext = {
    view: 'admin_payment_method_editor',
    mode: editingMethod ? 'edit' : 'create',
    method: editingMethod ? buildAdminPaymentMethodSummary(editingMethod) : undefined,
    form: {
      name: formData.name || undefined,
      icon: formData.icon,
      poll_interval: formData.poll_interval,
      script_length: formData.script.length,
      config_length: formData.config.length,
    },
    summary: adminPaymentMethodsPluginContext.summary,
  }
  const adminPaymentMethodImportPluginContext = {
    view: 'admin_payment_method_import',
    mode: isMarketImportMode ? 'market' : 'upload',
    target: selectedPackageTargetMethod
      ? buildAdminPaymentMethodSummary(selectedPackageTargetMethod)
      : undefined,
    preview: packagePreview?.resolved
      ? {
          name: packagePreview.resolved.name,
          version: packagePreview.resolved.version,
          entry: packagePreview.resolved.entry,
          icon: packagePreview.resolved.icon,
          script_bytes: packagePreview.resolved.script_bytes,
          webhook_count: packageWebhookCount,
          ready_to_import: packageConfigValid && packageMissingRequiredFields.length === 0,
        }
      : undefined,
    validation: {
      config_valid: packageConfigValid,
      missing_required_fields: packageMissingRequiredFields.map((field) => field.key),
      warning_count: marketWarnings.length,
    },
    market: adminPaymentMethodsPluginContext.market,
    summary: adminPaymentMethodsPluginContext.summary,
  }
  const packageTargetLabel =
    packageForm.target_id === 'new'
      ? locale === 'zh'
        ? '创建新支付方式'
        : 'Create new payment method'
      : selectedPackageTargetMethod?.name || '-'
  const packageTargetHint =
    packageForm.target_id === 'new'
      ? locale === 'zh'
        ? '导入完成后会创建一条新的支付方式记录，并写入当前解析出的脚本与配置默认值。'
        : 'The import will create a new payment method record with the resolved script defaults.'
      : marketTargetState?.installed === true
        ? locale === 'zh'
          ? '当前选择会直接更新这条已存在的支付方式，请确认名称、入口文件和配置覆盖范围。'
          : 'The selected target will be updated in place. Review name, entry file, and config overrides before import.'
        : locale === 'zh'
          ? '当前选择会将包内容导入到这条支付方式记录。'
          : 'The package will be imported into the selected payment method record.'
  const packageSchemaFieldCount = packageConfigSchema?.fields.length || 0
  const packageRequiredFieldCount =
    packageConfigSchema?.fields.filter((field) => field.required).length || 0
  const packageReadyToImport =
    !!packagePreview?.resolved && packageConfigValid && packageMissingRequiredFields.length === 0
  const packageReadinessLabel = !packagePreview?.resolved
    ? locale === 'zh'
      ? '等待预览'
      : 'Preview required'
    : !packageConfigValid
      ? locale === 'zh'
        ? '配置 JSON 无效'
        : 'Invalid config JSON'
      : packageMissingRequiredFields.length > 0
        ? locale === 'zh'
          ? `缺少 ${packageMissingRequiredFields.length} 项必填配置`
          : `${packageMissingRequiredFields.length} required config field(s) missing`
        : locale === 'zh'
          ? '可以导入'
          : 'Ready to import'

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

  const openImportDialog = () => {
    resetPackageImportState()
    setIsImportOpen(true)
  }

  useEffect(() => {
    if (!marketImportRequest || !marketImportRequestKey) {
      return
    }
    if (
      previewMarketPackageMutation.isPending ||
      marketImportPreviewKey === marketImportRequestKey
    ) {
      return
    }
    setMarketImportPreviewKey(marketImportRequestKey)
    previewMarketPackageMutation.mutate({
      ...marketImportRequest,
      payment_method_id:
        packageForm.target_id !== 'new' ? Number(packageForm.target_id) : undefined,
    })
  }, [
    marketImportPreviewKey,
    marketImportRequest,
    marketImportRequestKey,
    packageForm.target_id,
    previewMarketPackageMutation,
  ])

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

  const handlePreviewPackage = () => {
    if (!packageFile) {
      toast.error(t.admin.pmPackageSelectFileFirst)
      return
    }
    previewPackageMutation.mutate({
      file: packageFile,
      targetId: packageForm.target_id !== 'new' ? packageForm.target_id : undefined,
    })
  }

  const handleImportPackage = () => {
    if (!isMarketImportMode && !packageFile) {
      toast.error(t.admin.pmPackageSelectFileFirst)
      return
    }
    if (!packagePreview?.resolved) {
      toast.error(t.admin.pmPackagePreviewHint)
      return
    }

    const latestConfig = packageForm.config
    if (!packageConfigValid || !packageConfigObject) {
      toast.error(t.admin.pmInvalidConfigJson)
      return
    }
    if (packageMissingRequiredFields.length > 0) {
      toast.error(
        t.admin.pmPackageMissingRequiredConfig.replace(
          '{field}',
          packageMissingRequiredFields[0].label || packageMissingRequiredFields[0].key
        )
      )
      return
    }

    if (isMarketImportMode && marketImportRequest) {
      importMarketPackageMutation.mutate({
        request: marketImportRequest,
        payment_method_id:
          packageForm.target_id !== 'new' ? Number(packageForm.target_id) : undefined,
        payment_name: packageForm.name,
        payment_description: packageForm.description,
        icon: packageForm.icon,
        entry: packageForm.entry,
        config: latestConfig,
        poll_interval: packageForm.poll_interval || 30,
      })
      return
    }

    const payload = new FormData()
    payload.append('file', packageFile!)
    if (packageForm.target_id !== 'new') {
      payload.append('payment_method_id', packageForm.target_id)
    }
    payload.append('name', packageForm.name)
    payload.append('description', packageForm.description)
    payload.append('icon', packageForm.icon)
    payload.append('version', packageForm.version)
    payload.append('entry', packageForm.entry)
    payload.append('config', latestConfig)
    payload.append('poll_interval', String(packageForm.poll_interval || 30))
    uploadPackageMutation.mutate(payload)
  }

  const handlePackageConfigBlur = () => {
    setPackageForm((prev) => ({
      ...prev,
      config: tryFormatTextareaJSON(prev.config, '{}', formatHumanReadableJSONText),
    }))
  }

  const getIcon = (iconName: string) => {
    const Icon = iconMap[iconName] || CreditCard
    return <Icon className="h-5 w-5" />
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <h1 className="text-3xl font-bold">{t.admin.pmTitle}</h1>
        <div className="py-8 text-center">{t.common.loading}</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="admin.payment_methods.top" context={adminPaymentMethodsPluginContext} />
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.pmTitle}</h1>
          <p className="mt-1 text-muted-foreground">{t.admin.pmSubtitle}</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => initMutation.mutate()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.admin.pmInitBuiltin}
          </Button>
          {marketPluginImportHref ? (
            <Button variant="outline" asChild>
              <Link href={marketPluginImportHref}>
                <Package className="mr-2 h-4 w-4" />
                {t.admin.pmImportFromMarket}
              </Link>
            </Button>
          ) : null}
          <Button variant="outline" onClick={openImportDialog}>
            <FileUp className="mr-2 h-4 w-4" />
            {t.admin.pmImportPackage}
          </Button>
          <Button
            onClick={() => {
              resetForm()
              setIsCreateOpen(true)
            }}
          >
            <Plus className="mr-2 h-4 w-4" />
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
          methods.map((method: PaymentMethod, index: number) => {
            const rowExtensions = adminPaymentMethodActionExtensions[String(method.id)] || []
            return (
              <Card
                key={method.id}
                className={`transition-all duration-200 ${!method.enabled ? 'opacity-60' : ''} ${
                  dragIndex === index ? 'scale-[0.97] opacity-40 shadow-none' : ''
                } ${
                  overIndex === index && dragIndex !== null && dragIndex !== index
                    ? 'ring-2 ring-primary ring-offset-2'
                    : ''
                }`}
                draggable
                onDragStart={(e) => {
                  setDragIndex(index)
                  e.dataTransfer.effectAllowed = 'move'
                }}
                onDragOver={(e) => {
                  e.preventDefault()
                  e.dataTransfer.dropEffect = 'move'
                  setOverIndex(index)
                }}
                onDragLeave={(e) => {
                  if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                    setOverIndex((prev) => (prev === index ? null : prev))
                  }
                }}
                onDragEnd={() => {
                  const from = dragIndex
                  const to = overIndex
                  setDragIndex(null)
                  setOverIndex(null)
                  if (from === null || to === null || from === to) return
                  const reordered = [...methods]
                  const [moved] = reordered.splice(from, 1)
                  reordered.splice(to, 0, moved)
                  // Optimistic update
                  queryClient.setQueryData(['paymentMethods'], (old: any) => {
                    if (!old?.data?.items) return old
                    return { ...old, data: { ...old.data, items: reordered } }
                  })
                  reorderMutation.mutate(reordered.map((m: PaymentMethod) => m.id))
                }}
              >
                <CardContent className="space-y-3 p-4">
                  <div className="flex items-center gap-4">
                    <div className="cursor-grab text-muted-foreground transition-colors hover:text-foreground active:cursor-grabbing">
                      <GripVertical className="h-5 w-5" />
                    </div>
                    <div className="rounded-lg bg-muted p-2">
                      {getIcon(method.icon || 'CreditCard')}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="font-semibold">{method.name}</h3>
                      </div>
                      <p className="truncate text-sm text-muted-foreground">{method.description}</p>
                      <p className="mt-1 truncate text-xs text-muted-foreground">
                        {[
                          'JS',
                          method.package_name ? t.admin.pmPackageImportedBadge : null,
                          method.version ? `v${method.version}` : null,
                          method.package_name || null,
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </p>
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
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => setDeleteId(method.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                  {rowExtensions.length > 0 ? (
                    <div
                      className="flex justify-end"
                      onClick={(event) => event.stopPropagation()}
                      onMouseDown={(event) => event.stopPropagation()}
                      onDragStart={(event) => event.stopPropagation()}
                    >
                      <PluginExtensionList extensions={rowExtensions} display="inline" />
                    </div>
                  ) : null}
                </CardContent>
              </Card>
            )
          })
        )}
      </div>

      {/* 创建/编辑对话框 */}
      <Dialog
        open={isCreateOpen || !!editingMethod}
        onOpenChange={(open) => {
          if (!open) {
            setIsCreateOpen(false)
            setEditingMethod(null)
            setTestResult(null)
          }
        }}
      >
        <DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingMethod ? t.admin.pmEdit : t.admin.pmAdd}</DialogTitle>
            <DialogDescription>{t.admin.pmDialogDesc}</DialogDescription>
          </DialogHeader>

          <PluginSlot
            slot="admin.payment_methods.editor.top"
            context={adminPaymentMethodEditorPluginContext}
          />

          <Tabs defaultValue="basic" className="w-full">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="basic">
                <Settings className="mr-2 h-4 w-4" />
                {t.admin.pmTabBasic}
              </TabsTrigger>
              <TabsTrigger value="config">
                <CreditCard className="mr-2 h-4 w-4" />
                {t.admin.pmTabConfig}
              </TabsTrigger>
              <TabsTrigger value="script">
                <Code className="mr-2 h-4 w-4" />
                {t.admin.pmTabScript}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="basic" className="mt-4 space-y-4">
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
                  onChange={(e) =>
                    setFormData({ ...formData, poll_interval: parseInt(e.target.value) || 30 })
                  }
                  placeholder="30"
                />
                <p className="text-xs text-muted-foreground">{t.admin.pmPollIntervalHint}</p>
              </div>
              {editingMethod?.package_name ? (
                <Card className="bg-muted/40">
                  <CardHeader className="pb-3">
                    <CardTitle className="flex items-center gap-2 text-sm">
                      <Package className="h-4 w-4" />
                      {t.admin.pmPackageSource}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-2 text-xs text-muted-foreground">
                    <p>
                      <span className="font-medium text-foreground">{t.admin.pmPackageFile}:</span>{' '}
                      {editingMethod.package_name}
                    </p>
                    <p>
                      <span className="font-medium text-foreground">
                        {t.admin.pmPackageVersion}:
                      </span>{' '}
                      {editingMethod.version || '-'}
                    </p>
                    <p>
                      <span className="font-medium text-foreground">{t.admin.pmPackageEntry}:</span>{' '}
                      {editingMethod.package_entry || '-'}
                    </p>
                    <p className="break-all">
                      <span className="font-medium text-foreground">
                        {t.admin.pmPackageChecksum}:
                      </span>{' '}
                      {editingMethod.package_checksum || '-'}
                    </p>
                  </CardContent>
                </Card>
              ) : null}
              {editingMethod?.manifest || editingMethod?.package_name ? (
                <PaymentMethodWebhookPanel
                  manifest={editingMethod?.manifest}
                  paymentMethodId={editingMethod?.id}
                />
              ) : null}
            </TabsContent>

            <TabsContent value="config" className="mt-4 space-y-4">
              <ConfigEditor
                key={editingMethod?.id ?? 'new'}
                value={formData.config}
                onChange={(v) => setFormData({ ...formData, config: v })}
                flushRef={configFlushRef}
                labels={{
                  configJson: t.admin.pmConfigJson,
                  configFields: t.admin.pmConfigFields,
                  jsonEditor: t.admin.pmJsonEditor,
                  visualEditor: t.admin.pmVisualEditor,
                  invalidJson: t.admin.pmInvalidJson,
                  noFields: t.admin.pmNoFields,
                  addField: t.admin.pmAddField,
                }}
                cmTheme={resolvedTheme === 'dark' ? 'dark' : 'light'}
              />
            </TabsContent>

            <TabsContent value="script" className="mt-4 space-y-4">
              <div className="space-y-2">
                <Label>{t.admin.pmJsScript}</Label>
                <CodeMirror
                  value={formData.script}
                  extensions={[javascript()]}
                  onChange={(v) => setFormData({ ...formData, script: v })}
                  placeholder={t.admin.pmScriptPlaceholder}
                  height="300px"
                  theme={resolvedTheme === 'dark' ? 'dark' : 'light'}
                  className="overflow-hidden rounded-md border text-sm"
                />
                <div className="flex gap-2">
                  <Button variant="outline" onClick={handleTest} disabled={testMutation.isPending}>
                    <Play className="mr-2 h-4 w-4" />
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
                  <CardDescription className="text-xs">{t.admin.pmApiRefDesc}</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3 text-xs">
                  <div>
                    <p className="mb-1 font-semibold">{t.admin.pmRequiredCallbacks}</p>
                    <p>
                      <code>onGeneratePaymentCard(order, config)</code> -{' '}
                      {t.admin.pmGenerateCardHtml}
                    </p>
                    <p className="ml-4 text-muted-foreground">
                      {t.admin.pmReturns}
                      <code>{`{html, title?, description?, data?, cache_ttl?}`}</code>
                    </p>
                    <p className="ml-4 text-muted-foreground">
                      <code>cache_ttl</code>: {t.admin.pmCacheTtlDesc}
                    </p>
                    <p>
                      <code>onCheckPaymentStatus(order, config)</code> - {t.admin.pmCheckStatus}
                    </p>
                    <p className="ml-4 text-muted-foreground">
                      {t.admin.pmReturns}
                      <code>{`{paid: boolean, message?, transaction_id?, data?}`}</code>
                    </p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">{webhookSectionTitle}</p>
                    <p>
                      <code>onWebhook(hook, config)</code> - {webhookSectionDesc}
                    </p>
                    <p className="ml-4 text-muted-foreground">{webhookReturnsText}</p>
                    <p className="ml-4 text-muted-foreground">
                      <code>{webhookExampleURL}</code>
                    </p>
                    <p className="ml-4 text-muted-foreground">{webhookHintText}</p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">
                      AuraLogic.storage{' '}
                      <span className="font-normal text-muted-foreground">
                        ({t.admin.pmLocalStorage})
                      </span>
                    </p>
                    <p>
                      <code>get(key)</code> / <code>set(key, value)</code> /{' '}
                      <code>delete(key)</code>
                    </p>
                    <p>
                      <code>list()</code> / <code>clear()</code>
                    </p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">AuraLogic.order</p>
                    <p>
                      <code>get()</code> - {t.admin.pmGetOrder}
                    </p>
                    <p>
                      <code>getItems()</code> - {t.admin.pmGetOrderItems}
                    </p>
                    <p>
                      <code>getUser()</code> - {t.admin.pmGetOrderUser}
                    </p>
                    <p>
                      <code>updatePaymentData(data)</code> - {t.admin.pmUpdatePaymentData}
                    </p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">AuraLogic.config</p>
                    <p>
                      <code>get(key?, defaultValue?)</code> - {t.admin.pmGetConfigValue}
                    </p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">AuraLogic.utils</p>
                    <p>
                      <code>formatPrice(amount, currency)</code> - {t.admin.pmFormatPrice}
                    </p>
                    <p>
                      <code>formatDate(date, format?)</code> - {t.admin.pmFormatDate}
                    </p>
                    <p>
                      <code>generateId()</code> - {t.admin.pmGenerateUuid}
                    </p>
                    <p>
                      <code>md5(data)</code> / <code>hmacSHA256(data, secret)</code>
                    </p>
                    <p>
                      <code>base64Encode(data)</code> / <code>base64Decode(data)</code>
                    </p>
                    <p>
                      <code>jsonEncode(data)</code> / <code>jsonDecode(data)</code>
                    </p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">AuraLogic.http</p>
                    <p>
                      <code>get(url, headers?)</code> - {t.admin.pmGetRequest}
                    </p>
                    <p>
                      <code>post(url, body?, headers?)</code> - {t.admin.pmPostRequest}
                    </p>
                    <p>
                      <code>request(options)</code> - {t.admin.pmGeneralRequest}
                    </p>
                    <p className="text-muted-foreground">{t.admin.pmHttpReturns}</p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">AuraLogic.system</p>
                    <p>
                      <code>getTimestamp()</code> - {t.admin.pmGetTimestamp}
                    </p>
                    <p>
                      <code>getPaymentMethodInfo()</code> - {t.admin.pmGetMethodInfo}
                    </p>
                    <p>
                      <code>getWebhookUrl(hook)</code> - {webhookHintText}
                    </p>
                  </div>
                  <div>
                    <p className="mb-1 font-semibold">AuraLogic.webhook</p>
                    <p>
                      <code>enabled</code> / <code>key</code> / <code>method</code> /{' '}
                      <code>path</code>
                    </p>
                    <p>
                      <code>queryString</code> / <code>queryParams</code> / <code>headers</code>
                    </p>
                    <p>
                      <code>contentType</code> / <code>remoteAddr</code> / <code>bodyText</code> /{' '}
                      <code>bodyBase64</code>
                    </p>
                    <p>
                      <code>header(name)</code> / <code>query(name)</code> / <code>text()</code> /{' '}
                      <code>json()</code>
                    </p>
                  </div>
                  <div className="mt-3 border-t pt-3">
                    <p className="mb-1 font-semibold">{t.admin.pmThemeAdaptation}</p>
                    <p className="text-muted-foreground">{t.admin.pmThemeAdaptationDesc}</p>
                    <p className="mt-1">
                      <code className="text-green-600 dark:text-green-400">bg-muted</code>,{' '}
                      <code className="text-green-600 dark:text-green-400">
                        text-muted-foreground
                      </code>
                      , <code className="text-green-600 dark:text-green-400">text-primary</code>,{' '}
                      <code className="text-green-600 dark:text-green-400">border-border</code>
                    </p>
                    <p className="mt-1">
                      {t.admin.pmDarkModeStyles}
                      <code className="text-blue-600 dark:text-blue-400">
                        dark:bg-gray-800
                      </code>,{' '}
                      <code className="text-blue-600 dark:text-blue-400">dark:text-gray-100</code>
                    </p>
                  </div>
                  <div className="mt-3 border-t pt-3">
                    <p className="mb-1 font-semibold">{t.admin.pmMultiLanguage}</p>
                    <p className="text-muted-foreground">{t.admin.pmMultiLanguageDesc}</p>
                    <p className="mt-1">
                      <code className="text-purple-600 dark:text-purple-400">
                        &lt;span class="lang-zh"&gt;中文&lt;/span&gt;
                      </code>
                      <code className="ml-2 text-purple-600 dark:text-purple-400">
                        &lt;span class="lang-en"&gt;English&lt;/span&gt;
                      </code>
                    </p>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setIsCreateOpen(false)
                setEditingMethod(null)
                setTestResult(null)
              }}
            >
              {t.common.cancel}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {editingMethod ? t.common.save : t.common.create}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={isImportOpen}
        onOpenChange={(open) => {
          if (open) {
            setIsImportOpen(true)
          } else {
            closeImportDialog()
          }
        }}
      >
        <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {isMarketImportMode ? t.admin.pmImportFromMarket : t.admin.pmImportPackage}
            </DialogTitle>
            <DialogDescription>
              {isMarketImportMode ? t.admin.pmMarketImportDesc : t.admin.pmImportPackageDesc}
            </DialogDescription>
          </DialogHeader>

          <PluginSlot
            slot="admin.payment_methods.import.top"
            context={adminPaymentMethodImportPluginContext}
          />

          <div className="space-y-4">
            {!isMarketImportMode ? (
              <div className="space-y-2">
                <Label>{t.admin.pmPackageFile}</Label>
                <input
                  key={packageInputKey}
                  ref={packageFileInputRef}
                  type="file"
                  accept=".zip"
                  className="hidden"
                  onChange={(e) => {
                    setPackageFile(e.target.files?.[0] || null)
                    setPackagePreview(null)
                    setPackageForm((prev) => ({
                      ...createDefaultPackageForm(),
                      target_id: prev.target_id,
                    }))
                  }}
                />
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => packageFileInputRef.current?.click()}
                    disabled={previewPackageMutation.isPending || importPackagePending}
                  >
                    <FileUp className="mr-2 h-4 w-4" />
                    {t.admin.pluginUploadChooseFile}
                  </Button>
                  <span className="break-all text-sm text-muted-foreground">
                    {packageFile?.name || t.admin.pluginUploadNoFileSelected}
                  </span>
                </div>
                <p className="text-xs text-muted-foreground">{t.admin.pmPackagePreviewHint}</p>
              </div>
            ) : null}

            <div className="space-y-2">
              <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
                <div className="space-y-2">
                  <Label>{t.admin.pmPackageTarget}</Label>
                  <Select
                    value={packageForm.target_id}
                    onValueChange={(value) =>
                      setPackageForm((prev) => ({ ...prev, target_id: value }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="new">{t.admin.pmPackageTargetNew}</SelectItem>
                      {methods.map((method: PaymentMethod) => (
                        <SelectItem key={method.id} value={String(method.id)}>
                          {method.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                {!isMarketImportMode ? (
                  <div className="flex md:self-end">
                    <Button
                      type="button"
                      variant="outline"
                      className="h-10 w-full md:min-w-[112px]"
                      onClick={handlePreviewPackage}
                      disabled={
                        !packageFile || previewPackageMutation.isPending || importPackagePending
                      }
                    >
                      {previewPackageMutation.isPending ? (
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      ) : (
                        <Package className="mr-2 h-4 w-4" />
                      )}
                      {locale === 'zh' ? '预览' : 'Preview'}
                    </Button>
                  </div>
                ) : null}
              </div>
              <p className="text-xs text-muted-foreground">{t.admin.pmPackageTargetHint}</p>
            </div>

            {packagePreview?.resolved ? (
              <>
                <Card className="bg-muted/30">
                  <CardHeader className="pb-3">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="space-y-1">
                        <CardTitle className="text-sm">{t.admin.pmPackageSummary}</CardTitle>
                        <CardDescription>
                          {isMarketImportMode
                            ? locale === 'zh'
                              ? '这里展示的是最终会写入支付方式的解析结果。确认无误后可直接导入。'
                              : 'This is the resolved payload that will be written into the payment method record. Import once the final values look correct.'
                            : locale === 'zh'
                              ? '本地包预览已经完成。确认入口文件、版本和配置默认值后即可导入。'
                              : 'Local package preview is ready. Review entry file, version, and config defaults before import.'}
                        </CardDescription>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        {[
                          packageReadinessLabel,
                          isMarketImportMode ? marketImportModeLabel : null,
                          packageTargetLabel,
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </p>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
                      {[
                        {
                          label: t.admin.pmPackageScriptBytes,
                          value: formatScriptBytes(packagePreview.resolved.script_bytes),
                        },
                        {
                          label: t.admin.pmPackageWebhookCount,
                          value: String(packageWebhookCount),
                        },
                        {
                          label: locale === 'zh' ? '配置字段' : 'Config fields',
                          value: String(packageSchemaFieldCount),
                        },
                        {
                          label: locale === 'zh' ? '必填缺失' : 'Required missing',
                          value: String(packageMissingRequiredFields.length),
                        },
                      ].map((item) => (
                        <div
                          key={item.label}
                          className="rounded-lg border bg-background/80 px-3 py-2"
                        >
                          <p className="text-[11px] uppercase tracking-wide text-muted-foreground">
                            {item.label}
                          </p>
                          <p className="mt-1 break-all text-sm font-medium">{item.value}</p>
                        </div>
                      ))}
                    </div>
                    <div className="grid gap-3 text-sm md:grid-cols-2">
                      <div>
                        <p className="text-xs text-muted-foreground">{t.admin.pmName}</p>
                        <p className="break-all font-medium">
                          {packageManifestName || packagePreview.resolved.name || '-'}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">{t.admin.pmPackageVersion}</p>
                        <p className="font-medium">{packagePreview.resolved.version || '-'}</p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">{t.admin.pmPackageEntry}</p>
                        <p className="break-all font-medium">
                          {packagePreview.resolved.entry || '-'}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {locale === 'zh' ? '最终轮询间隔' : 'Final poll interval'}
                        </p>
                        <p className="font-medium">{packageForm.poll_interval || 30}</p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {locale === 'zh' ? '导入目标' : 'Import target'}
                        </p>
                        <p className="break-all font-medium">{packageTargetLabel}</p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {locale === 'zh' ? '目标当前版本' : 'Current target version'}
                        </p>
                        <p className="break-all font-medium">
                          {String(
                            marketTargetState?.current_version ||
                              selectedPackageTargetMethod?.version ||
                              ''
                          ).trim() || '-'}
                        </p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {locale === 'zh' ? '配置准备状态' : 'Config readiness'}
                        </p>
                        <p className="break-all font-medium">{packageReadinessLabel}</p>
                      </div>
                      <div>
                        <p className="text-xs text-muted-foreground">
                          {locale === 'zh' ? 'Schema 必填项' : 'Schema required fields'}
                        </p>
                        <p className="font-medium">{packageRequiredFieldCount}</p>
                      </div>
                      <div className="md:col-span-2">
                        <p className="text-xs text-muted-foreground">{t.admin.pmPackageChecksum}</p>
                        <p className="break-all font-medium">{packagePreview.resolved.checksum}</p>
                      </div>
                    </div>
                    <p className="text-xs text-muted-foreground">{packageTargetHint}</p>
                    {packageManifestDescription ? (
                      <div className="rounded-lg border bg-background/70 px-3 py-2">
                        <p className="text-xs text-muted-foreground">
                          {locale === 'zh' ? '包描述' : 'Package description'}
                        </p>
                        <p className="mt-1 text-sm text-foreground/90">
                          {packageManifestDescription}
                        </p>
                      </div>
                    ) : null}
                    {isMarketImportMode && marketWarnings.length > 0 ? (
                      <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 dark:border-amber-500/40 dark:bg-amber-950/20">
                        <p className="text-xs font-medium text-amber-700 dark:text-amber-300">
                          {t.admin.pmMarketWarnings}
                        </p>
                        <div className="mt-2 space-y-1">
                          {marketWarnings.map((warning, index) => (
                            <p
                              key={`${warning}-${index}`}
                              className="text-xs text-amber-700/90 dark:text-amber-200/90"
                            >
                              {warning}
                            </p>
                          ))}
                        </div>
                      </div>
                    ) : null}
                    {packageMissingRequiredFields.length > 0 ? (
                      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm">
                        <p className="font-medium text-destructive">
                          {locale === 'zh'
                            ? '还有必填配置未填写'
                            : 'Required config is still missing'}
                        </p>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {(locale === 'zh' ? '缺失字段：' : 'Missing fields: ') +
                            packageMissingRequiredFields
                              .slice(0, 6)
                              .map((field) => field.label || field.key)
                              .join(locale === 'zh' ? '、' : ', ')}
                        </p>
                      </div>
                    ) : packageReadyToImport ? (
                      <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3 text-sm dark:border-emerald-500/40 dark:bg-emerald-950/20">
                        <p className="font-medium text-emerald-700 dark:text-emerald-300">
                          {locale === 'zh'
                            ? '预览、配置和目标选择都已就绪，可以直接导入。'
                            : 'Preview, config, and target selection are ready. You can import now.'}
                        </p>
                      </div>
                    ) : null}
                  </CardContent>
                </Card>

                <PaymentMethodWebhookPanel
                  webhooks={packagePreview.manifest?.webhooks}
                  paymentMethodId={packageImportTargetPaymentMethodId}
                />

                <div className="grid gap-4 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label>{t.admin.pmName}</Label>
                    <Input
                      value={packageForm.name}
                      onChange={(e) =>
                        setPackageForm((prev) => ({ ...prev, name: e.target.value }))
                      }
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t.admin.pmPackageVersion}</Label>
                    <Input
                      value={packageForm.version}
                      onChange={(e) =>
                        setPackageForm((prev) => ({ ...prev, version: e.target.value }))
                      }
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label>{t.admin.pmDescription}</Label>
                    <Textarea
                      value={packageForm.description}
                      onChange={(e) =>
                        setPackageForm((prev) => ({ ...prev, description: e.target.value }))
                      }
                      rows={3}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label>{t.admin.pmIcon}</Label>
                    <Select
                      value={packageForm.icon}
                      onValueChange={(value) =>
                        setPackageForm((prev) => ({ ...prev, icon: value }))
                      }
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
                      value={packageForm.poll_interval}
                      onChange={(e) =>
                        setPackageForm((prev) => ({
                          ...prev,
                          poll_interval: parseInt(e.target.value) || 30,
                        }))
                      }
                    />
                  </div>
                  <div className="space-y-2 md:col-span-2">
                    <Label>{t.admin.pmPackageEntry}</Label>
                    <Input
                      value={packageForm.entry}
                      onChange={(e) =>
                        setPackageForm((prev) => ({ ...prev, entry: e.target.value }))
                      }
                      placeholder="index.js"
                    />
                    <p className="text-xs text-muted-foreground">{t.admin.pmPackageEntryHint}</p>
                  </div>
                </div>

                <div className="space-y-3">
                  {packageConfigSchema ? (
                    <PluginJSONSchemaEditor
                      title={packageConfigSchema.title || t.admin.pluginConfigPresetEditor}
                      description={
                        packageConfigSchema.description || t.admin.pluginConfigPresetEditorDesc
                      }
                      schema={packageConfigSchema}
                      value={packageForm.config}
                      onChange={(value) => setPackageForm((prev) => ({ ...prev, config: value }))}
                      disabled={!packageConfigValid}
                      disabledReason={
                        !packageConfigValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined
                      }
                      t={t}
                    />
                  ) : null}

                  <PluginJSONObjectEditor
                    title={
                      packageConfigSchema
                        ? t.admin.pluginConfigVisualEditorExtra
                        : t.admin.pluginConfigVisualEditor
                    }
                    description={
                      packageConfigSchema
                        ? t.admin.pluginConfigVisualEditorExtraDesc
                        : t.admin.pluginConfigVisualEditorDesc
                    }
                    value={packageForm.config}
                    onChange={(value) => setPackageForm((prev) => ({ ...prev, config: value }))}
                    excludedKeys={packageConfigSchema?.fields.map((field) => field.key) || []}
                    emptyMessage={t.admin.pluginJsonObjectEditorNoExtraFields}
                    disabled={!packageConfigValid}
                    disabledReason={
                      !packageConfigValid ? t.admin.pluginJsonObjectEditorInvalidRaw : undefined
                    }
                    t={t}
                  />

                  <details
                    className="space-y-2 rounded-md border border-input p-3"
                    open={!packageConfigValid}
                  >
                    <summary className="cursor-pointer text-sm font-medium">
                      {t.admin.pluginAdvancedJsonEditor} · {t.admin.pmConfigJson}
                    </summary>
                    <div className="mt-3 space-y-2">
                      <Label>{t.admin.pmConfigJson}</Label>
                      <p className="text-xs text-muted-foreground">
                        {t.admin.pluginAdvancedJsonEditorDesc}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {t.admin.pluginJsonAutoFormatHint}
                      </p>
                      <Textarea
                        value={packageForm.config}
                        onChange={(e) =>
                          setPackageForm((prev) => ({ ...prev, config: e.target.value }))
                        }
                        onBlur={handlePackageConfigBlur}
                        className="font-mono text-xs"
                        rows={6}
                      />
                    </div>
                  </details>
                </div>
              </>
            ) : null}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={closeImportDialog}>
              {t.common.cancel}
            </Button>
            <Button
              onClick={handleImportPackage}
              disabled={!packagePreview?.resolved || importPackagePending}
            >
              {importPackagePending ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : (
                <FileUp className="mr-2 h-4 w-4" />
              )}
              {isMarketImportMode ? t.admin.pmImportFromMarket : t.admin.pmImportPackage}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 删除确认 */}
      <AlertDialog open={deleteId !== null} onOpenChange={() => setDeleteId(null)}>
        <AlertDialogContent className="max-w-lg">
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>{t.admin.pmDeleteDesc}</AlertDialogDescription>
          </AlertDialogHeader>
          {deleteMethod ? (
            <div className="space-y-3">
              <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm">
                <p className="text-xs text-muted-foreground">
                  {[
                    t.common.delete,
                    `#${deleteMethod.id}`,
                    deleteMethod.type === 'builtin'
                      ? t.order.builtinPaymentMethod
                      : t.order.customPaymentMethod,
                    deleteMethod.enabled ? t.admin.enabled : t.admin.disabled,
                    deleteMethod.version ? `v${deleteMethod.version}` : null,
                    deleteMethod.package_name ? t.admin.pmPackageImportedBadge : null,
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
                <p className="mt-2 break-words font-medium">{deleteMethod.name}</p>
                {deleteMethod.description ? (
                  <p className="mt-1 break-words text-xs text-muted-foreground">
                    {deleteMethod.description}
                  </p>
                ) : null}
              </div>
              {deleteMethod.package_name ? (
                <div className="rounded-md border border-input/60 bg-muted/10 p-3 text-sm">
                  <p className="break-all font-mono text-xs text-muted-foreground">
                    {deleteMethod.package_name}
                  </p>
                </div>
              ) : null}
            </div>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteId && deleteMutation.mutate(deleteId)}
              className="bg-red-600 hover:bg-red-700"
              disabled={deleteMutation.isPending || deleteMethod === null}
            >
              {deleteMutation.isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
