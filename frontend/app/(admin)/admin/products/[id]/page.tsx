'use client'

import { useState, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getAdminProduct, createProduct, updateProduct, uploadImage, batchCreateProductBindings, deleteProductBinding, updateProductInventoryMode, getInventories, replaceProductBindings, getVirtualInventories, getProductVirtualInventoryBindings, createProductVirtualInventoryBinding, deleteProductVirtualInventoryBinding, saveProductVirtualVariantBindings } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { ProductVariantInventory, VariantInventoryBinding } from '@/components/admin/product-variant-inventory'
import { ProductVirtualVariantInventory, VirtualVariantInventoryBinding } from '@/components/admin/product-virtual-variant-inventory'
import toast from 'react-hot-toast'
import { ArrowLeft, Save, Plus, Trash2, Upload, Loader2, Image as ImageIcon, Database, FileText, RefreshCw, Eye, Pencil } from 'lucide-react'
import Link from 'next/link'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { MarkdownMessage } from '@/components/ui/markdown-message'

// 虚拟库存绑定卡片组件
function VirtualInventoryBindingCard({ productId, isNew }: { productId: number | null; isNew: boolean }) {
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [addDialogOpen, setAddDialogOpen] = useState(false)
  const [selectedVirtualInventoryId, setSelectedVirtualInventoryId] = useState<string>('')

  // 获取商品的虚拟库存绑定
  const { data: bindingsData, isLoading: bindingsLoading, refetch: refetchBindings } = useQuery({
    queryKey: ['productVirtualInventoryBindings', productId],
    queryFn: () => getProductVirtualInventoryBindings(productId!),
    enabled: !!productId && !isNew,
  })

  // 获取所有虚拟库存列表
  const { data: virtualInventoriesData } = useQuery({
    queryKey: ['virtualInventories', 1, 100],
    queryFn: () => getVirtualInventories({ page: 1, limit: 100 }),
  })

  // 创建绑定
  const createBindingMutation = useMutation({
    mutationFn: (data: { virtual_inventory_id: number }) =>
      createProductVirtualInventoryBinding(productId!, data),
    onSuccess: () => {
      toast.success(t.admin.bindingSuccess)
      setAddDialogOpen(false)
      setSelectedVirtualInventoryId('')
      refetchBindings()
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.bindingFailed)
    },
  })

  // 删除绑定
  const deleteBindingMutation = useMutation({
    mutationFn: (bindingId: number) => deleteProductVirtualInventoryBinding(productId!, bindingId),
    onSuccess: () => {
      toast.success(t.admin.unbindSuccess)
      refetchBindings()
    },
    onError: (error: Error) => {
      toast.error(error.message || t.admin.unbindFailed)
    },
  })

  const bindings = bindingsData?.data?.bindings || []
  const virtualInventories = virtualInventoriesData?.data?.list || []

  // 过滤掉已绑定的库存
  const availableInventories = virtualInventories.filter(
    (vi: any) => !bindings.some((b: any) => b.virtual_inventory_id === vi.id)
  )

  const handleAddBinding = () => {
    if (!selectedVirtualInventoryId) {
      toast.error(t.admin.selectVirtualFirst)
      return
    }
    createBindingMutation.mutate({ virtual_inventory_id: parseInt(selectedVirtualInventoryId) })
  }

  if (isNew) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            {t.admin.virtualInventoryBinding}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p>{t.admin.saveProductFirst}</p>
            <p className="text-sm mt-2">{t.admin.virtualProductBindingHint}</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            {t.admin.virtualInventoryBinding}
          </CardTitle>
          <div className="flex gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => refetchBindings()}>
              <RefreshCw className="h-4 w-4 mr-1" />
              {t.admin.refreshBtn}
            </Button>
            <Button type="button" size="sm" onClick={() => setAddDialogOpen(true)}>
              <Plus className="h-4 w-4 mr-1" />
              {t.admin.bindVirtualInventory}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {bindingsLoading ? (
          <div className="text-center py-8">{t.common.loading}</div>
        ) : bindings.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <FileText className="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p>{t.admin.noVirtualBinding}</p>
            <p className="text-sm mt-2">{t.admin.noVirtualBindingHint}</p>
            <div className="flex gap-2 justify-center mt-4">
              <Button variant="outline" asChild>
                <Link href="/admin/inventories?tab=virtual">
                  {t.admin.goCreateVirtual}
                </Link>
              </Button>
              <Button type="button" onClick={() => setAddDialogOpen(true)}>
                <Plus className="h-4 w-4 mr-1" />
                {t.admin.bindVirtualInventory}
              </Button>
            </div>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t.admin.inventoryNameCol}</TableHead>
                <TableHead>{t.admin.skuCol}</TableHead>
                <TableHead>{t.admin.availableCol}</TableHead>
                <TableHead>{t.admin.reservedCol}</TableHead>
                <TableHead>{t.admin.soldCol}</TableHead>
                <TableHead>{t.admin.statusCol}</TableHead>
                <TableHead>{t.admin.actionCol}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {bindings.map((binding: any) => (
                <TableRow key={binding.id}>
                  <TableCell className="font-medium">
                    {binding.virtual_inventory?.name || 'N/A'}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {binding.virtual_inventory?.sku || '-'}
                  </TableCell>
                  <TableCell>
                    <Badge variant={binding.virtual_inventory?.available > 0 ? 'default' : 'destructive'}>
                      {binding.virtual_inventory?.available || 0}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {binding.virtual_inventory?.reserved || 0}
                  </TableCell>
                  <TableCell>
                    {binding.virtual_inventory?.sold || 0}
                  </TableCell>
                  <TableCell>
                    {binding.virtual_inventory?.is_active ? (
                      <Badge variant="default">{t.admin.enabledStatus}</Badge>
                    ) : (
                      <Badge variant="secondary">{t.admin.disabledStatus}</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button size="sm" variant="destructive">
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>{t.admin.confirmUnbind}</AlertDialogTitle>
                          <AlertDialogDescription>
                            {t.admin.unbindDesc}
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={() => deleteBindingMutation.mutate(binding.id)}
                          >
                            {t.admin.confirmUnbindBtn}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {/* 添加绑定对话框 */}
        <Dialog open={addDialogOpen} onOpenChange={setAddDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t.admin.bindVirtualTitle}</DialogTitle>
              <DialogDescription>
                {t.admin.selectVirtualPool}
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4">
              {availableInventories.length === 0 ? (
                <div className="text-center py-4 text-muted-foreground">
                  <p>{t.admin.noAvailableVirtual}</p>
                  <p className="text-sm mt-2">{t.admin.createVirtualFirst}</p>
                  <Button variant="outline" className="mt-4" asChild>
                    <Link href="/admin/inventories?tab=virtual">
                      {t.admin.goCreate}
                    </Link>
                  </Button>
                </div>
              ) : (
                <div className="space-y-2">
                  <Label>{t.admin.selectVirtualLabel}</Label>
                  <Select
                    value={selectedVirtualInventoryId}
                    onValueChange={setSelectedVirtualInventoryId}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t.admin.selectVirtualPlaceholder} />
                    </SelectTrigger>
                    <SelectContent>
                      {availableInventories.map((vi: any) => (
                        <SelectItem key={vi.id} value={vi.id.toString()}>
                          {vi.name} {vi.sku ? `(${vi.sku})` : ''} - {t.admin.availableLabel}: {vi.available}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={() => setAddDialogOpen(false)}>
                {t.common.cancel}
              </Button>
              <Button
                onClick={handleAddBinding}
                disabled={createBindingMutation.isPending || !selectedVirtualInventoryId}
              >
                {createBindingMutation.isPending ? t.admin.binding : t.admin.confirmBind}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  )
}

interface ProductForm {
  sku: string
  name: string
  product_code: string
  product_type: 'physical' | 'virtual'
  description: string
  short_description: string
  category: string
  tags: string[]
  price: number
  original_price: number
  stock: number
  max_purchase_limit: number
  images: Array<{ url: string; alt: string; is_primary: boolean }>
  attributes: Array<{ name: string; values: string[]; mode: 'user_select' | 'blind_box'; valuesInput?: string }>
  status: string
  sort_order: number
  is_featured: boolean
  is_recommended: boolean
  auto_delivery: boolean
  remark: string
  // 规格与库存配置
  variant_mode: 'user_select' | 'blind_box'  // 规格模式
  variant_inventory_bindings: VariantInventoryBinding[]  // 规格库存绑定（实体商品）
  virtual_variant_inventory_bindings: VirtualVariantInventoryBinding[]  // 规格库存绑定（虚拟商品）
}

export default function ProductEditPage() {
  const params = useParams()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const isNew = params.id === 'new'
  usePageTitle(isNew ? t.pageTitle.adminProductNew : t.pageTitle.adminProductEdit)
  const productId = isNew ? null : Number(params.id)

  const [form, setForm] = useState<ProductForm>({
    sku: '',
    name: '',
    product_code: '',
    product_type: 'physical',
    description: '',
    short_description: '',
    category: '',
    tags: [],
    price: 0,
    original_price: 0,
    stock: 0,
    max_purchase_limit: 0,
    images: [],
    attributes: [],
    status: 'draft',
    sort_order: 0,
    is_featured: false,
    is_recommended: false,
    auto_delivery: false,
    remark: '',
    // 规格与库存配置
    variant_mode: 'user_select',
    variant_inventory_bindings: [],
    virtual_variant_inventory_bindings: [],
  })

  const [tagInput, setTagInput] = useState('')
  const [newImageUrl, setNewImageUrl] = useState('')
  const [newImageAlt, setNewImageAlt] = useState('')
  const [isUploading, setIsUploading] = useState(false)
  const [selectKey, setSelectKey] = useState(0) // 用于强制 Select 重新渲染
  const [descPreview, setDescPreview] = useState(false)
  const [isFormDataLoaded, setIsFormDataLoaded] = useState(isNew) // 追踪表单数据是否已加载

  // 获取商品详情（编辑模式）
  const { data: productData, isLoading } = useQuery({
    queryKey: ['adminProduct', productId],
    queryFn: () => getAdminProduct(productId!),
    enabled: !isNew && productId !== null,
    refetchOnMount: 'always', // 每次进入页面都重新获取数据
    staleTime: 0, // 数据立即过期，确保总是获取最新数据
  })

  // 获取所有库存列表（用于匹配绑定关系）
  const { data: inventoriesData } = useQuery({
    queryKey: ['inventories', 1, 200],
    queryFn: () => getInventories({ page: 1, limit: 200 }),
    // 创建和编辑商品时都需要库存列表
  })

  // 获取所有虚拟库存列表
  const { data: virtualInventoriesData } = useQuery({
    queryKey: ['virtualInventories', 1, 200],
    queryFn: () => getVirtualInventories({ page: 1, limit: 200 }),
  })

  // 填充表单数据
  useEffect(() => {
    // 如果是新建模式，不需要填充数据
    if (isNew) {
      return
    }

    if (productData?.data) {
      const product = productData.data

      const formData = {
        sku: product.sku || '',
        name: product.name || '',
        product_code: product.product_code || product.productCode || '',
        product_type: (product.product_type || product.productType || 'physical') as 'physical' | 'virtual',
        description: product.description || '',
        short_description: product.short_description || product.shortDescription || '',
        category: product.category || '',
        tags: product.tags || [],
        price: product.price ?? 0,
        original_price: product.original_price ?? product.originalPrice ?? 0,
        stock: product.stock ?? 0,
        max_purchase_limit: product.max_purchase_limit ?? product.maxPurchaseLimit ?? 0,
        images: product.images || [],
        attributes: (product.attributes || []).map((attr: any) => {
          // 确保 values 是字符串数组
          const values = Array.isArray(attr.values)
            ? attr.values.map((v: any) => String(v || ''))
            : []
          return {
            name: String(attr.name || ''),
            values: values,
            mode: attr.mode || 'user_select', // 兼容旧��据，默认为用户自选
            valuesInput: values.join(','), // 初始化原始输入字符串
          }
        }),
        status: product.status || 'draft',
        sort_order: product.sort_order ?? product.sortOrder ?? 0,
        is_featured: product.is_featured ?? product.isFeatured ?? false,
        is_recommended: product.is_recommended ?? product.isRecommended ?? false,
        auto_delivery: product.auto_delivery ?? false,
        remark: product.remark || '',
        // 规格与库存配置
        variant_mode: (product.inventory_mode === 'random' ? 'blind_box' : 'user_select') as 'user_select' | 'blind_box',
        variant_inventory_bindings: (() => {
          // 辅助函数：标准化属性对象的key排序，确保一致性
          const normalizeAttributes = (attrs: Record<string, string>) => {
            const sorted = Object.keys(attrs).sort().reduce((obj: Record<string, string>, key) => {
              obj[key] = attrs[key]
              return obj
            }, {})
            return sorted
          }

          // 1. 从API返回的简化绑定中提取数据
          const existingBindingsMap = new Map()
          const bindings = product.inventory_bindings || []

          bindings.forEach((binding: any) => {
            // 直接从 attributes 字段读取规格组合（新格式）
            let attributes = binding.attributes || {}

            // 兼容旧格式：如果 attributes 为空但 notes 有数据，尝试从 notes 解析
            if (Object.keys(attributes).length === 0 && binding.notes) {
              if (binding.notes.startsWith('规格: ')) {
                try {
                  const jsonStr = binding.notes.substring(4).trim()
                  attributes = JSON.parse(jsonStr)
                } catch (e) {
                  // 解析失败，使用空对象
                }
              }
            }

            // 标准化attributes以确保一致性
            const normalizedAttrs = normalizeAttributes(attributes)
            const key = JSON.stringify(normalizedAttrs)

            existingBindingsMap.set(key, {
              attributes: normalizedAttrs,
              inventory_id: binding.inventory_id,
              priority: binding.priority || 1,
            })
          })

          // 2. 根据商品属性生成所有规格组合
          const generateAllVariants = () => {
            if (!product.attributes || product.attributes.length === 0) {
              return [{}]
            }

            const validAttrs = product.attributes.filter((attr: any) => attr.name && attr.values && attr.values.length > 0)
            if (validAttrs.length === 0) {
              return [{}]
            }

            const cartesian = (arr: any[][]): any[][] => {
              return arr.reduce(
                (a, b) => a.flatMap((x) => b.map((y) => [...x, y])),
                [[]]
              )
            }

            const attrArrays = validAttrs.map((attr: any) =>
              attr.values.map((value: string) => ({ [attr.name]: value }))
            )

            const combinations = cartesian(attrArrays)
            return combinations.map((combo) => normalizeAttributes(Object.assign({}, ...combo)))
          }

          const allVariants = generateAllVariants()

          // 3. 合并：已存在的绑定保留配置，新的规格使用默认值
          const result = allVariants.map(attrs => {
            const key = JSON.stringify(attrs)
            const existing = existingBindingsMap.get(key)
            return existing || {
              attributes: attrs,
              inventory_id: null,
              priority: 1,
            }
          })

          return result
        })(),
        // 虚拟库存规格绑定
        virtual_variant_inventory_bindings: (() => {
          const normalizeAttributes = (attrs: Record<string, string>) => {
            const sorted = Object.keys(attrs).sort().reduce((obj: Record<string, string>, key) => {
              obj[key] = attrs[key]
              return obj
            }, {})
            return sorted
          }

          // 从虚拟库存绑定中提取数据
          const existingBindingsMap = new Map()
          const bindings = product.virtual_inventory_bindings || []

          for (const binding of bindings) {
            let attrs = {}
            if (binding.attributes && typeof binding.attributes === 'object') {
              attrs = normalizeAttributes(binding.attributes)
            }
            const key = JSON.stringify(attrs)
            existingBindingsMap.set(key, {
              attributes: attrs,
              virtual_inventory_id: binding.virtual_inventory_id,
              is_random: binding.is_random ?? false,
              priority: binding.priority ?? 1,
            })
          }

          // 生成所有规格组合
          const generateAllVariants = () => {
            if (!product.attributes || product.attributes.length === 0) {
              return [{}]
            }

            const validAttrs = product.attributes.filter((attr: any) => attr.name && attr.values && attr.values.length > 0)
            if (validAttrs.length === 0) {
              return [{}]
            }

            const cartesian = (arr: any[][]): any[][] => {
              return arr.reduce(
                (a, b) => a.flatMap((x) => b.map((y) => [...x, y])),
                [[]]
              )
            }

            const attrArrays = validAttrs.map((attr: any) =>
              attr.values.map((value: string) => ({ [attr.name]: value }))
            )

            const combinations = cartesian(attrArrays)
            return combinations.map((combo) => normalizeAttributes(Object.assign({}, ...combo)))
          }

          const allVariants = generateAllVariants()

          const result = allVariants.map(attrs => {
            const key = JSON.stringify(attrs)
            const existing = existingBindingsMap.get(key)
            return existing || {
              attributes: attrs,
              virtual_inventory_id: null,
              is_random: false,
              priority: 1,
            }
          })

          return result
        })(),
      }

      setForm(formData)
      setIsFormDataLoaded(true) // 标记数据已加载

      // 强制 Select 组件重新渲染
      setSelectKey(prev => prev + 1)
    }
  }, [productData, productId, isNew])

  // 保存商品
  // 辅助函数：标准化并序列化attributes（与后端保持一致）
  const normalizeAndStringify = (attrs: Record<string, string>) => {
    // 按key排序
    const sorted = Object.keys(attrs).sort().reduce((obj: Record<string, string>, key) => {
      obj[key] = attrs[key]
      return obj
    }, {})
    return JSON.stringify(sorted)
  }

  const saveMutation = useMutation({
    mutationFn: async (data: any) => {
      if (isNew) {
        // 创建商品
        const response = await createProduct(data)
        const newProductId = response.data.id

        // 设置库存模式（将variant_mode转换为inventory_mode）
        const inventoryMode = form.variant_mode === 'blind_box' ? 'random' : 'fixed'
        await updateProductInventoryMode(newProductId, inventoryMode)

        // 实体商品：创建规格库存绑定（去重并批量创建）
        if (form.product_type === 'physical' && form.variant_inventory_bindings && form.variant_inventory_bindings.length > 0) {
          const uniqueBindings = new Map()

          // 使用标准化的规格组合JSON字符串作为key进行去重
          for (const binding of form.variant_inventory_bindings) {
            if (binding.inventory_id) {
              const key = normalizeAndStringify(binding.attributes)
              if (!uniqueBindings.has(key)) {
                uniqueBindings.set(key, binding)
              }
            }
          }

          // 批量创建绑定（一次请求）
          const bindingsToCreate = Array.from(uniqueBindings.values()).map(binding => {
            const notesStr = normalizeAndStringify(binding.attributes)
            return {
              inventory_id: binding.inventory_id!,
              is_random: form.variant_mode === 'blind_box',
              priority: binding.priority || 1,
              notes: notesStr,  // 直接发送JSON字符串
            }
          })

          if (bindingsToCreate.length > 0) {
            await batchCreateProductBindings(newProductId, bindingsToCreate)
          }
        }

        // 虚拟商品：创建规格-虚拟库存绑定
        if (form.product_type === 'virtual' && form.virtual_variant_inventory_bindings && form.virtual_variant_inventory_bindings.length > 0) {
          const virtualBindings = form.virtual_variant_inventory_bindings
            .filter(b => b.virtual_inventory_id !== null)
            .map(b => ({
              attributes: b.attributes,
              virtual_inventory_id: b.virtual_inventory_id,
              is_random: b.is_random || false,
              priority: b.priority || 1,
            }))

          if (virtualBindings.length > 0) {
            await saveProductVirtualVariantBindings(newProductId, virtualBindings)
          }
        }

        return response
      } else {
        // 编辑模式：更新商品信息
        const response = await updateProduct(productId!, data)

        // 更新库存模式
        const inventoryMode = form.variant_mode === 'blind_box' ? 'random' : 'fixed'
        await updateProductInventoryMode(productId!, inventoryMode)

        // 实体商品：处理库存绑定的更新
        if (form.product_type === 'physical') {
          const uniqueBindings = new Map()

          // 使用标准化的规格组合JSON字符串作为key进行去重
          if (form.variant_inventory_bindings && form.variant_inventory_bindings.length > 0) {
            for (const binding of form.variant_inventory_bindings) {
              if (binding.inventory_id) {
                const key = normalizeAndStringify(binding.attributes)
                if (!uniqueBindings.has(key)) {
                  uniqueBindings.set(key, binding)
                }
              }
            }
          }

          // 批量替换绑定（一次请求：删除所有旧的 + 创建新的）
          const bindingsToCreate = Array.from(uniqueBindings.values()).map(binding => {
            const notesStr = normalizeAndStringify(binding.attributes)
            return {
              inventory_id: binding.inventory_id!,
              is_random: form.variant_mode === 'blind_box',
              priority: binding.priority || 1,
              notes: notesStr,  // 直接发送JSON字符串
            }
          })

          // 总是调用替换API，即使是空数组（用于删除所有绑定）
          await replaceProductBindings(productId!, bindingsToCreate)
        }

        // 虚拟商品：处理虚拟库存绑定的更新
        if (form.product_type === 'virtual') {
          const virtualBindings = (form.virtual_variant_inventory_bindings || [])
            .filter(b => b.virtual_inventory_id !== null)
            .map(b => ({
              attributes: b.attributes,
              virtual_inventory_id: b.virtual_inventory_id,
              is_random: b.is_random || false,
              priority: b.priority || 1,
            }))

          // 总是调用保存API来更新绑定（会自动删除旧的并创建新的）
          await saveProductVirtualVariantBindings(productId!, virtualBindings)
        }

        return response
      }
    },
    onSuccess: () => {
      toast.success(isNew ? t.admin.productCreated : t.admin.productUpdated)
      router.push('/admin/products')
    },
    onError: (error: Error) => {
      const msg = (error?.message || '').toString()
      const lower = msg.toLowerCase()
      // Backend uses a stable message; also guard against legacy raw DB errors.
      if (
        lower.includes('sku already exists') ||
        lower.includes('products.sku') ||
        (lower.includes('unique') && lower.includes('sku'))
      ) {
        toast.error(t.admin.skuAlreadyExists)
        return
      }
      toast.error(`${t.admin.productSaveFailed}: ${msg}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    // 验证必填字段
    if (!form.sku || !form.name) {
      toast.error(t.admin.fillSkuAndName)
      return
    }

    // 验证价格
    if (form.price <= 0) {
      toast.error(t.admin.priceMustBePositive)
      return
    }

    // 实体商品：验证规格库存配置
    if (form.product_type === 'physical' && form.variant_inventory_bindings && form.variant_inventory_bindings.length > 0) {
      const hasUnassigned = form.variant_inventory_bindings.some(v => !v.inventory_id)
      if (hasUnassigned) {
        toast.error(t.admin.configAllSpecInventory)
        return
      }
    }

    // 虚拟商品：验证虚拟库存规格配置
    if (form.product_type === 'virtual' && form.virtual_variant_inventory_bindings && form.virtual_variant_inventory_bindings.length > 0) {
      const hasUnassigned = form.virtual_variant_inventory_bindings.some(v => !v.virtual_inventory_id)
      if (hasUnassigned) {
        toast.error(t.admin.configAllSpecVirtualInventory)
        return
      }
    }

    // 提交数据（不包含variant_inventory_bindings，因为绑定会在创建/更新商品后单独处理）
    const submitData = { ...form }
    delete (submitData as any).variant_inventory_bindings
    delete (submitData as any).virtual_variant_inventory_bindings
    delete (submitData as any).variant_mode

    // 清理 attributes 中的 valuesInput 字���（仅用于前端输入，不发送到后端）
    submitData.attributes = submitData.attributes.map(attr => ({
      name: attr.name,
      values: attr.values,
      mode: attr.mode,
    }))

    saveMutation.mutate(submitData)
  }

  const addTag = () => {
    if (tagInput && !form.tags.includes(tagInput)) {
      setForm({ ...form, tags: [...form.tags, tagInput] })
      setTagInput('')
    }
  }

  const removeTag = (tag: string) => {
    setForm({ ...form, tags: form.tags.filter(t => t !== tag) })
  }

  const addImage = () => {
    if (newImageUrl) {
      setForm({
        ...form,
        images: [
          ...form.images,
          {
            url: newImageUrl,
            alt: newImageAlt,
            is_primary: form.images.length === 0,
          },
        ],
      })
      setNewImageUrl('')
      setNewImageAlt('')
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    // 检查文件类型
    if (!file.type.startsWith('image/')) {
      toast.error(t.admin.selectImageFile)
      return
    }

    // 检查文件大小（5MB）
    if (file.size > 5 * 1024 * 1024) {
      toast.error(t.admin.imageSizeLimit)
      return
    }

    setIsUploading(true)

    try {
      const response = await uploadImage(file)
      const imageUrl = response.data.url

      // 自动添加到图片列表
      setForm({
        ...form,
        images: [
          ...form.images,
          {
            url: imageUrl,
            alt: file.name.replace(/\.[^/.]+$/, ''), // 使用文件名作为alt
            is_primary: form.images.length === 0,
          },
        ],
      })

      toast.success(t.admin.imageUploadSuccess)

      // 清空文件输入
      e.target.value = ''
    } catch (error: any) {
      toast.error(error.message || t.admin.imageUploadFailed)
    } finally {
      setIsUploading(false)
    }
  }

  const removeImage = (index: number) => {
    const newImages = form.images.filter((_, i) => i !== index)
    setForm({ ...form, images: newImages })
  }

  const setPrimaryImage = (index: number) => {
    const newImages = form.images.map((img, i) => ({
      ...img,
      is_primary: i === index,
    }))
    setForm({ ...form, images: newImages })
  }

  const addAttribute = () => {
    setForm({
      ...form,
      attributes: [...form.attributes, { name: '', values: [], mode: 'user_select', valuesInput: '' }],
    })
  }

  const updateAttribute = (index: number, field: 'name' | 'values' | 'mode' | 'valuesInput', value: any) => {
    const newAttributes = [...form.attributes]
    if (field === 'valuesInput' && typeof value === 'string') {
      // 保存原始输入字符串
      newAttributes[index].valuesInput = value
      // 实时解析为数组（用于生成规格组合）
      newAttributes[index].values = value.split(',').map(v => v.trim()).filter(v => v)
    } else if (field === 'name') {
      newAttributes[index].name = String(value || '')
    } else if (field === 'mode') {
      newAttributes[index].mode = value as 'user_select' | 'blind_box'
    }
    setForm({ ...form, attributes: newAttributes })
  }

  const removeAttribute = (index: number) => {
    setForm({
      ...form,
      attributes: form.attributes.filter((_, i) => i !== index),
    })
  }

  // 加载状态
  if (!isNew && isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin mx-auto mb-4" />
          <p className="text-muted-foreground">{t.admin.loadingProductInfo}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="sm" asChild>
          <Link href="/admin/products">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.common.back}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold">
          {isNew ? t.admin.addProductTitle : t.admin.editProduct}
        </h1>
        {!isNew && form.product_type === 'virtual' && (
          <Button variant="outline" asChild>
            <Link href={`/admin/products/${productId}/virtual-stock`}>
              <Database className="mr-2 h-4 w-4" />
              {t.admin.virtualStockManageBtn}
            </Link>
          </Button>
        )}
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.productBasicInfo}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="sku">
                  SKU <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="sku"
                  value={form.sku}
                  onChange={(e) => setForm({ ...form, sku: e.target.value })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="name">
                  {t.admin.productNameLabel} <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="name"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  required
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="product_code">
                {t.admin.productCodeLabel}
                <span className="text-sm text-muted-foreground ml-2">{t.admin.productCodeHint}</span>
              </Label>
              <Input
                id="product_code"
                value={form.product_code}
                onChange={(e) => setForm({ ...form, product_code: e.target.value.toUpperCase() })}
                placeholder={t.admin.productCodeInputPlaceholder}
                maxLength={20}
              />
              <p className="text-xs text-muted-foreground">
                {t.admin.productCodeTip}<strong>{t.admin.productCodeTipFormat}</strong>
                <br />
                {t.admin.productCodeExample}
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="short_description">{t.admin.shortDescLabel}</Label>
              <Input
                id="short_description"
                value={form.short_description}
                onChange={(e) => setForm({ ...form, short_description: e.target.value })}
                placeholder={t.admin.shortDescPlaceholder}
              />
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="description">{t.admin.detailDescLabel}</Label>
                <div className="flex items-center gap-1 rounded-md border border-border p-0.5">
                  <button
                    type="button"
                    onClick={() => setDescPreview(false)}
                    className={`inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors ${!descPreview ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                  >
                    <Pencil className="h-3 w-3" />
                    {t.admin.descEditTab}
                  </button>
                  <button
                    type="button"
                    onClick={() => setDescPreview(true)}
                    className={`inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-medium transition-colors ${descPreview ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'}`}
                  >
                    <Eye className="h-3 w-3" />
                    {t.admin.descPreviewTab}
                  </button>
                </div>
              </div>
              {descPreview ? (
                <div className="min-h-[156px] rounded-md border border-border p-3">
                  {form.description ? (
                    <MarkdownMessage content={form.description} className="text-sm" allowHtml />
                  ) : (
                    <p className="text-sm text-muted-foreground">{t.admin.descPreviewEmpty}</p>
                  )}
                </div>
              ) : (
                <>
                  <Textarea
                    id="description"
                    value={form.description}
                    onChange={(e) => setForm({ ...form, description: e.target.value })}
                    rows={6}
                  />
                  <p className="text-xs text-muted-foreground">{t.admin.descMarkdownHint}</p>
                </>
              )}
            </div>

            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <Label htmlFor="category">{t.admin.categoryLabel}</Label>
                <Input
                  id="category"
                  value={form.category}
                  onChange={(e) => setForm({ ...form, category: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="product_type">{t.admin.productTypeLabel}</Label>
                <Select key={`type-${selectKey}`} value={form.product_type} onValueChange={(value: 'physical' | 'virtual') => setForm({ ...form, product_type: value })}>
                  <SelectTrigger>
                    <SelectValue placeholder={t.admin.productTypeLabel} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="physical">{t.admin.physicalProduct}</SelectItem>
                    <SelectItem value="virtual">{t.admin.virtualProduct}</SelectItem>
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  {t.admin.virtualProductHint}
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="status">{t.admin.statusLabel}</Label>
                <Select key={selectKey} value={form.status} onValueChange={(value) => setForm({ ...form, status: value })}>
                  <SelectTrigger>
                    <SelectValue placeholder={t.admin.statusLabel} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="draft">{t.admin.statusDraft}</SelectItem>
                    <SelectItem value="active">{t.admin.statusActive}</SelectItem>
                    <SelectItem value="inactive">{t.admin.statusInactive}</SelectItem>
                    <SelectItem value="out_of_stock">{t.admin.statusOutOfStock}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label>{t.admin.tagsLabel}</Label>
              <div className="flex gap-2">
                <Input
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyPress={(e) => e.key === 'Enter' && (e.preventDefault(), addTag())}
                  placeholder={t.admin.tagInputPlaceholder}
                />
                <Button type="button" onClick={addTag}>
                  <Plus className="h-4 w-4" />
                </Button>
              </div>
              <div className="flex flex-wrap gap-2 mt-2">
                {form.tags.map((tag) => (
                  <div
                    key={tag}
                    className="bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300 px-3 py-1 rounded-full text-sm flex items-center gap-2"
                  >
                    {tag}
                    <button
                      type="button"
                      onClick={() => removeTag(tag)}
                      className="hover:text-blue-900 dark:hover:text-blue-200"
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.priceAndStock}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="price">
                  {t.admin.priceLabel} (¥) <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="price"
                  type="number"
                  step="0.01"
                  min="0.01"
                  value={form.price}
                  onChange={(e) => setForm({ ...form, price: parseFloat(e.target.value) || 0 })}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="original_price">{t.admin.originalPriceLabel} (¥)</Label>
                <Input
                  id="original_price"
                  type="number"
                  step="0.01"
                  min="0"
                  value={form.original_price}
                  onChange={(e) => setForm({ ...form, original_price: parseFloat(e.target.value) || 0 })}
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="max_purchase_limit">{t.admin.maxPurchaseLimitLabel}</Label>
              <Input
                id="max_purchase_limit"
                type="number"
                min="0"
                value={form.max_purchase_limit}
                onChange={(e) => setForm({ ...form, max_purchase_limit: parseInt(e.target.value) || 0 })}
                placeholder={t.admin.maxPurchaseLimitPlaceholder}
                className="w-64"
              />
              <p className="text-xs text-muted-foreground">
                {t.admin.maxPurchaseLimitHint}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{t.admin.productImages}</CardTitle>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => document.getElementById('imageUpload')?.click()}
                  disabled={isUploading}
                >
                  {isUploading ? (
                    <>
                      <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                      {t.admin.uploading}
                    </>
                  ) : (
                    <>
                      <Upload className="h-4 w-4 mr-2" />
                      {t.admin.uploadImage}
                    </>
                  )}
                </Button>
                <input
                  id="imageUpload"
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={handleFileUpload}
                />
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex gap-2">
              <Input
                placeholder={t.admin.inputImageUrl}
                value={newImageUrl}
                onChange={(e) => setNewImageUrl(e.target.value)}
              />
              <Input
                placeholder={t.admin.imageAlt}
                value={newImageAlt}
                onChange={(e) => setNewImageAlt(e.target.value)}
                className="w-48"
              />
              <Button type="button" onClick={addImage}>
                <Plus className="h-4 w-4" />
              </Button>
            </div>
            {form.images.length === 0 ? (
              <div className="text-center py-12 border-2 border-dashed rounded-lg">
                <ImageIcon className="w-12 h-12 mx-auto mb-3 text-muted-foreground" />
                <p className="text-sm text-muted-foreground mb-3">{t.admin.noImages}</p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => document.getElementById('imageUpload')?.click()}
                >
                  <Upload className="h-4 w-4 mr-2" />
                  {t.admin.uploadFirstImage}
                </Button>
              </div>
            ) : (
              <div className="grid grid-cols-4 gap-4">
                {form.images.map((image, index) => (
                  <div key={index} className="relative border rounded p-2 group">
                    <div className="relative">
                      <img
                        src={image.url}
                        alt={image.alt}
                        className="w-full h-32 object-cover rounded"
                      />
                      {image.is_primary && (
                        <div className="absolute top-1 left-1">
                          <span className="bg-blue-500 text-white text-xs px-2 py-0.5 rounded">
                            {t.admin.primaryImage}
                          </span>
                        </div>
                      )}
                    </div>
                    <div className="mt-2 space-y-1">
                      <div className="flex items-center gap-2">
                        <Switch
                          checked={image.is_primary}
                          onCheckedChange={() => setPrimaryImage(index)}
                        />
                        <span className="text-xs">{t.admin.setPrimary}</span>
                      </div>
                      <Button
                        type="button"
                        variant="destructive"
                        size="sm"
                        onClick={() => removeImage(index)}
                        className="w-full"
                      >
                        <Trash2 className="h-3 w-3 mr-1" />
                        {t.common.delete}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{t.admin.productSpecs}</CardTitle>
              <Button type="button" size="sm" onClick={addAttribute}>
                <Plus className="mr-2 h-4 w-4" />
                {t.admin.addSpec}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            {form.attributes.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground">
                <p>{t.admin.noSpecs}</p>
                <p className="text-sm mt-2">{t.admin.noSpecsExample}</p>
              </div>
            ) : (
              form.attributes.map((attr, index) => (
                <div key={index} className="border rounded-lg p-4 space-y-3">
                  <div className="flex items-start gap-3">
                    <div className="flex-1 space-y-3">
                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-2">
                          <Label htmlFor={`attr-name-${index}`}>{t.admin.specName}</Label>
                          <Input
                            id={`attr-name-${index}`}
                            placeholder={t.admin.specNamePlaceholder}
                            value={attr.name}
                            onChange={(e) => updateAttribute(index, 'name', e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor={`attr-mode-${index}`}>{t.admin.specType}</Label>
                          <Select
                            value={attr.mode || 'user_select'}
                            onValueChange={(value: 'user_select' | 'blind_box') => updateAttribute(index, 'mode', value)}
                          >
                            <SelectTrigger id={`attr-mode-${index}`}>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="user_select">{t.admin.userSelect}</SelectItem>
                              <SelectItem value="blind_box">{t.admin.blindBox}</SelectItem>
                            </SelectContent>
                          </Select>
                          <p className="text-xs text-muted-foreground">
                            {attr.mode === 'blind_box'
                              ? t.admin.blindBoxHint
                              : t.admin.userSelectHint}
                          </p>
                        </div>
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor={`attr-values-${index}`}>
                          {t.admin.specValues}
                          <span className="text-sm text-muted-foreground ml-2">
                            {t.admin.specValuesHint}
                          </span>
                        </Label>
                        <Input
                          id={`attr-values-${index}`}
                          placeholder={t.admin.specValuesPlaceholder}
                          value={attr.valuesInput || (Array.isArray(attr.values) ? attr.values.join(',') : '')}
                          onChange={(e) => updateAttribute(index, 'valuesInput', e.target.value)}
                        />
                        {Array.isArray(attr.values) && attr.values.length > 0 && (
                          <div className="flex flex-wrap gap-2 mt-2">
                            {attr.values.map((value, vIndex) => (
                              <div
                                key={vIndex}
                                className="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300 px-3 py-1 rounded text-sm"
                              >
                                {String(value)}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      onClick={() => removeAttribute(index)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.otherSettings}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-3 gap-4">
              <div className="flex items-center space-x-2">
                <Switch
                  id="is_featured"
                  checked={form.is_featured}
                  onCheckedChange={(checked) => setForm({ ...form, is_featured: checked })}
                />
                <Label htmlFor="is_featured">{t.admin.featuredProduct}</Label>
              </div>
              <div className="flex items-center space-x-2">
                <Switch
                  id="is_recommended"
                  checked={form.is_recommended}
                  onCheckedChange={(checked) => setForm({ ...form, is_recommended: checked })}
                />
                <Label htmlFor="is_recommended">{t.admin.recommendedProduct}</Label>
              </div>
              {form.product_type === 'virtual' && (
                <div className="flex items-center space-x-2">
                  <Switch
                    id="auto_delivery"
                    checked={form.auto_delivery}
                    onCheckedChange={(checked) => setForm({ ...form, auto_delivery: checked })}
                  />
                  <Label htmlFor="auto_delivery">{t.admin.autoDelivery}</Label>
                </div>
              )}
              <div className="space-y-2">
                <Label htmlFor="sort_order">{t.admin.sortOrder}</Label>
                <Input
                  id="sort_order"
                  type="number"
                  value={form.sort_order}
                  onChange={(e) => setForm({ ...form, sort_order: parseInt(e.target.value) || 0 })}
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="remark">{t.admin.remarkLabel}</Label>
              <Textarea
                id="remark"
                value={form.remark}
                onChange={(e) => setForm({ ...form, remark: e.target.value })}
                rows={3}
              />
            </div>
          </CardContent>
        </Card>

        {/* 规格与库存配置：根据商品类型显示不同的界面 */}
        {/* 只有在表单数据加载完成后才渲染，避免用空数据初始化 */}
        {isFormDataLoaded && form.product_type === 'physical' && (
          <ProductVariantInventory
            key={selectKey}
            attributes={form.attributes.map(attr => ({
              name: attr.name,
              values: attr.values,
              mode: attr.mode
            }))}
            variantMode={form.variant_mode}
            bindings={form.variant_inventory_bindings}
            inventories={inventoriesData?.data?.items || []}
            onVariantModeChange={(mode) => setForm({ ...form, variant_mode: mode })}
            onBindingsChange={(bindings) => setForm({ ...form, variant_inventory_bindings: bindings })}
          />
        )}

        {/* 虚拟商品库存管理 - 规格与虚拟库存绑定（与实体商品相同的绑定制） */}
        {isFormDataLoaded && form.product_type === 'virtual' && (
          <ProductVirtualVariantInventory
            key={`virtual-${selectKey}`}
            attributes={form.attributes.map(attr => ({
              name: attr.name,
              values: attr.values,
              mode: attr.mode
            }))}
            variantMode={form.variant_mode}
            bindings={form.virtual_variant_inventory_bindings}
            virtualInventories={virtualInventoriesData?.data?.list || []}
            onBindingsChange={(bindings) => setForm({ ...form, virtual_variant_inventory_bindings: bindings })}
          />
        )}

        <div className="flex justify-end gap-4">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/products">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.saving : t.common.save}
          </Button>
        </div>
      </form>
    </div>
  )
}
